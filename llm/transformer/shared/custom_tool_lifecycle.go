package shared

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// Custom tool lifecycle: preserve (native path elsewhere) | bridge-to-function |
// drop (FilterOutResponseCustomToolMessages) + rehydrate after provider response.
// Orchestrator and outbound adapters must call this module rather than
// re-implementing custom↔function conversion.

var freeformCustomToolFunctionParameters = json.RawMessage(`{
	"type":"object",
	"properties":{
		"input":{
			"type":"string",
			"description":"The complete freeform input for the custom tool. Do not wrap it in Markdown."
		}
	},
	"required":["input"],
	"additionalProperties":false
}`)

type freeformCustomToolBridgeSpec struct {
	Name      string
	Namespace string
}

// FreeformCustomToolBridge maps bridged function names back to Responses/Chat
// custom tool identity for response/stream rehydration.
type FreeformCustomToolBridge struct {
	byFunctionName map[string]freeformCustomToolBridgeSpec
}

// HasOpenAICustomTools reports whether the request carries OpenAI freeform
// custom tool declarations or calls (Responses or Chat shapes).
func HasOpenAICustomTools(req *llm.Request) bool {
	if req == nil {
		return false
	}

	for _, tool := range req.Tools {
		if tool.Type == llm.ToolTypeResponsesCustomTool || tool.ResponseCustomTool != nil ||
			(tool.Type == "custom" && tool.OpenAIChatCustomTool != nil) {
			return true
		}
	}

	for _, message := range req.Messages {
		for _, toolCall := range message.ToolCalls {
			if toolCall.Type == llm.ToolTypeResponsesCustomTool || toolCall.ResponseCustomToolCall != nil ||
				(toolCall.Type == "custom" && toolCall.OpenAIChatCustomToolCall != nil) {
				return true
			}
		}
	}

	return false
}

// NewFreeformCustomToolBridgeFromFunctionNames builds a rehydrate-only bridge
// (function name → custom leaf name). Used by tests and thin adapters that
// already know the mapping without re-running BridgeOpenAICustomToolsToFunctions.
func NewFreeformCustomToolBridgeFromFunctionNames(functionToLeaf map[string]string) *FreeformCustomToolBridge {
	bridge := &FreeformCustomToolBridge{
		byFunctionName: make(map[string]freeformCustomToolBridgeSpec, len(functionToLeaf)),
	}
	for functionName, leaf := range functionToLeaf {
		bridge.byFunctionName[functionName] = freeformCustomToolBridgeSpec{Name: leaf}
	}
	return bridge
}

// BridgeOpenAICustomToolsToFunctions rewrites freeform custom tools/calls into
// function tools for targets that cannot carry native custom tools. Records
// LossyDowngrade for custom.format when present. Returns nil bridge when nothing
// was converted.
func BridgeOpenAICustomToolsToFunctions(
	req *llm.Request,
	targetFormat llm.APIFormat,
) (*llm.Request, *FreeformCustomToolBridge, error) {
	if req == nil || !HasOpenAICustomTools(req) {
		return req, nil, nil
	}

	bridge := &FreeformCustomToolBridge{
		byFunctionName: make(map[string]freeformCustomToolBridgeSpec),
	}
	existingFunctions := make(map[string]struct{})
	for _, tool := range req.Tools {
		if tool.Type == llm.ToolTypeFunction && tool.Function.Name != "" {
			existingFunctions[tool.Function.Name] = struct{}{}
		}
	}

	cloned := *req
	cloned.Tools = append([]llm.Tool(nil), req.Tools...)
	for i, tool := range cloned.Tools {
		spec, description, ok := freeformCustomToolDeclaration(tool)
		if !ok {
			continue
		}
		functionName := freeformCustomToolFunctionName(spec)
		if err := bridge.register(functionName, spec, existingFunctions); err != nil {
			return nil, nil, err
		}
		cloned.Tools[i] = llm.Tool{
			Type: llm.ToolTypeFunction,
			Function: llm.Function{
				Name:        functionName,
				Description: bridgedFreeformToolDescription(description),
				Parameters:  append(json.RawMessage(nil), freeformCustomToolFunctionParameters...),
			},
			CacheControl: tool.CacheControl,
		}
		llm.AddLossyDowngradeIfPresent(
			req,
			req.APIFormat,
			"tools[].type=custom.format",
			targetFormat,
			freeformCustomToolHasFormat(tool),
		)
	}

	cloned.Messages = append([]llm.Message(nil), req.Messages...)
	for messageIndex, message := range cloned.Messages {
		if len(message.ToolCalls) == 0 {
			continue
		}
		message.ToolCalls = append([]llm.ToolCall(nil), message.ToolCalls...)
		changed := false
		for toolCallIndex, toolCall := range message.ToolCalls {
			spec, input, callID, ok := freeformCustomToolCall(toolCall)
			if !ok {
				continue
			}
			functionName := freeformCustomToolFunctionName(spec)
			if err := bridge.register(functionName, spec, existingFunctions); err != nil {
				return nil, nil, err
			}
			arguments, err := json.Marshal(struct {
				Input string `json:"input"`
			}{Input: input})
			if err != nil {
				return nil, nil, fmt.Errorf("failed to encode custom tool input: %w", err)
			}

			converted := toolCall
			converted.Type = llm.ToolTypeFunction
			if converted.ID == "" {
				converted.ID = callID
			}
			converted.Function = llm.FunctionCall{
				Name:      functionName,
				Arguments: string(arguments),
			}
			converted.ResponseCustomToolCall = nil
			converted.OpenAIChatCustomToolCall = nil
			message.ToolCalls[toolCallIndex] = converted
			changed = true
		}
		if changed {
			cloned.Messages[messageIndex] = message
		}
	}

	if req.ToolChoice != nil && req.ToolChoice.OpenAIChatCustomToolChoice != nil {
		name := req.ToolChoice.OpenAIChatCustomToolChoice.Name
		spec := freeformCustomToolBridgeSpec{Name: name}
		if err := bridge.register(name, spec, existingFunctions); err != nil {
			return nil, nil, err
		}
		cloned.ToolChoice = &llm.ToolChoice{
			NamedToolChoice: &llm.NamedToolChoice{
				Type: llm.ToolTypeFunction,
				Function: llm.ToolFunction{
					Name: name,
				},
			},
		}
	}

	if len(bridge.byFunctionName) == 0 {
		return req, nil, nil
	}
	return &cloned, bridge, nil
}

func (b *FreeformCustomToolBridge) register(
	functionName string,
	spec freeformCustomToolBridgeSpec,
	existingFunctions map[string]struct{},
) error {
	if functionName == "" || spec.Name == "" {
		return fmt.Errorf("%w: custom tool name must not be empty", transformer.ErrInvalidRequest)
	}
	if _, exists := existingFunctions[functionName]; exists {
		return fmt.Errorf(
			"%w: custom tool %q conflicts with an existing function tool",
			transformer.ErrInvalidRequest,
			functionName,
		)
	}
	if existing, exists := b.byFunctionName[functionName]; exists && existing != spec {
		return fmt.Errorf(
			"%w: custom tool function bridge name %q is ambiguous",
			transformer.ErrInvalidRequest,
			functionName,
		)
	}
	b.byFunctionName[functionName] = spec
	return nil
}

func freeformCustomToolDeclaration(tool llm.Tool) (freeformCustomToolBridgeSpec, string, bool) {
	if tool.ResponseCustomTool != nil {
		return freeformCustomToolBridgeSpec{Name: tool.ResponseCustomTool.Name}, tool.ResponseCustomTool.Description, true
	}
	if tool.OpenAIChatCustomTool != nil {
		description := ""
		if tool.OpenAIChatCustomTool.Description != nil {
			description = *tool.OpenAIChatCustomTool.Description
		}
		return freeformCustomToolBridgeSpec{Name: tool.OpenAIChatCustomTool.Name}, description, true
	}
	return freeformCustomToolBridgeSpec{}, "", false
}

func freeformCustomToolCall(toolCall llm.ToolCall) (freeformCustomToolBridgeSpec, string, string, bool) {
	if toolCall.ResponseCustomToolCall != nil {
		call := toolCall.ResponseCustomToolCall
		return freeformCustomToolBridgeSpec{Name: call.Name, Namespace: call.Namespace}, call.Input, call.CallID, true
	}
	if toolCall.OpenAIChatCustomToolCall != nil {
		call := toolCall.OpenAIChatCustomToolCall
		return freeformCustomToolBridgeSpec{Name: call.Name}, call.Input, toolCall.ID, true
	}
	return freeformCustomToolBridgeSpec{}, "", "", false
}

func freeformCustomToolHasFormat(tool llm.Tool) bool {
	if tool.ResponseCustomTool != nil {
		return tool.ResponseCustomTool.Format != nil
	}
	if tool.OpenAIChatCustomTool != nil {
		return len(tool.OpenAIChatCustomTool.Format) > 0
	}
	return false
}

func freeformCustomToolFunctionName(spec freeformCustomToolBridgeSpec) string {
	return llm.FunctionCall{Name: spec.Name, Namespace: spec.Namespace}.CompositeName()
}

func bridgedFreeformToolDescription(description string) string {
	const bridgeInstruction = `AxonHub compatibility bridge: provide the complete freeform tool input in the "input" string property.`
	if strings.TrimSpace(description) == "" {
		return bridgeInstruction
	}
	return description + "\n\n" + bridgeInstruction
}

// RehydrateResponse rewrites provider function tool calls that were produced
// for bridged custom tools back to Responses custom tool call shape.
func (b *FreeformCustomToolBridge) RehydrateResponse(response *llm.Response) error {
	if b == nil || response == nil || response == llm.DoneResponse {
		return nil
	}
	for choiceIndex := range response.Choices {
		for _, message := range []*llm.Message{response.Choices[choiceIndex].Message, response.Choices[choiceIndex].Delta} {
			if message == nil {
				continue
			}
			for toolCallIndex := range message.ToolCalls {
				toolCall := &message.ToolCalls[toolCallIndex]
				spec, ok := b.byFunctionName[toolCall.Function.Name]
				if !ok || toolCall.ResponseCustomToolCall != nil || toolCall.OpenAIChatCustomToolCall != nil {
					continue
				}
				input, err := decodeBridgedFreeformInput(toolCall.Function.Arguments)
				if err != nil {
					return fmt.Errorf("failed to decode bridged custom tool %q response: %w", spec.Name, err)
				}
				rehydrateFreeformToolCall(toolCall, spec, input)
			}
		}
	}
	return nil
}

func rehydrateFreeformToolCall(toolCall *llm.ToolCall, spec freeformCustomToolBridgeSpec, input string) {
	toolCall.Type = llm.ToolTypeResponsesCustomTool
	toolCall.ResponseCustomToolCall = &llm.ResponseCustomToolCall{
		CallID:    toolCall.ID,
		Name:      spec.Name,
		Namespace: spec.Namespace,
		Input:     input,
	}
	toolCall.OpenAIChatCustomToolCall = nil
	toolCall.Function = llm.FunctionCall{}
}

func decodeBridgedFreeformInput(arguments string) (string, error) {
	var payload struct {
		Input *string `json:"input"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return "", err
	}
	if payload.Input == nil {
		return "", fmt.Errorf("missing required string field input")
	}
	return *payload.Input, nil
}

type freeformCustomToolBridgeStream struct {
	source  streams.Stream[*llm.Response]
	bridge  *FreeformCustomToolBridge
	states  map[freeformCustomToolStreamKey]*freeformCustomToolStreamState
	current *llm.Response
	err     error
}

type freeformCustomToolStreamState struct {
	spec      freeformCustomToolBridgeSpec
	callID    string
	arguments strings.Builder
	emitted   bool
}

type freeformCustomToolStreamKey struct {
	choiceIndex   int
	toolCallIndex int
}

// NewFreeformCustomToolBridgeStream wraps a response stream to rehydrate
// bridged function tool calls into custom tool calls for the client.
func NewFreeformCustomToolBridgeStream(
	source streams.Stream[*llm.Response],
	bridge *FreeformCustomToolBridge,
) streams.Stream[*llm.Response] {
	if bridge == nil {
		return source
	}
	return &freeformCustomToolBridgeStream{
		source: source,
		bridge: bridge,
		states: make(map[freeformCustomToolStreamKey]*freeformCustomToolStreamState),
	}
}

func (s *freeformCustomToolBridgeStream) Next() bool {
	if s.err != nil {
		return false
	}
	if !s.source.Next() {
		if sourceErr := s.source.Err(); sourceErr != nil {
			s.err = sourceErr
			return false
		}
		s.err = s.validateCompletedCalls()
		return false
	}

	response := s.source.Current()
	if err := s.rehydrateChunk(response); err != nil {
		s.err = err
		return false
	}
	if responseHasFinishReason(response) {
		if err := s.validateCompletedCalls(); err != nil {
			s.err = err
			return false
		}
	}
	s.current = response
	return true
}

func (s *freeformCustomToolBridgeStream) rehydrateChunk(response *llm.Response) error {
	if response == nil || response == llm.DoneResponse {
		return nil
	}
	for choiceIndex := range response.Choices {
		for _, message := range []*llm.Message{response.Choices[choiceIndex].Message, response.Choices[choiceIndex].Delta} {
			if message == nil {
				continue
			}
			for toolCallIndex := range message.ToolCalls {
				toolCall := &message.ToolCalls[toolCallIndex]
				key := freeformCustomToolStreamKey{
					choiceIndex:   response.Choices[choiceIndex].Index,
					toolCallIndex: toolCall.Index,
				}
				state := s.states[key]
				if toolCall.Function.Name != "" {
					spec, ok := s.bridge.byFunctionName[toolCall.Function.Name]
					if !ok {
						continue
					}
					if state == nil {
						state = &freeformCustomToolStreamState{spec: spec, callID: toolCall.ID}
						s.states[key] = state
					}
				}
				if state == nil {
					continue
				}
				if toolCall.ID != "" {
					state.callID = toolCall.ID
				}
				toolCall.ID = state.callID

				state.arguments.WriteString(toolCall.Function.Arguments)
				inputDelta := ""
				if !state.emitted {
					if input, err := decodeBridgedFreeformInput(state.arguments.String()); err == nil {
						inputDelta = input
						state.emitted = true
					}
				}
				rehydrateFreeformToolCall(toolCall, state.spec, inputDelta)
			}
		}
	}
	return nil
}

func (s *freeformCustomToolBridgeStream) validateCompletedCalls() error {
	for _, state := range s.states {
		input, err := decodeBridgedFreeformInput(state.arguments.String())
		if err != nil {
			return fmt.Errorf("failed to decode streamed bridged custom tool %q response: %w", state.spec.Name, err)
		}
		if !state.emitted && input != "" {
			return fmt.Errorf("bridged custom tool %q input was not emitted", state.spec.Name)
		}
	}
	return nil
}

func responseHasFinishReason(response *llm.Response) bool {
	if response == nil || response == llm.DoneResponse {
		return response == llm.DoneResponse
	}
	for _, choice := range response.Choices {
		if choice.FinishReason != nil {
			return true
		}
	}
	return false
}

func (s *freeformCustomToolBridgeStream) Current() *llm.Response {
	return s.current
}

func (s *freeformCustomToolBridgeStream) Err() error {
	return s.err
}

func (s *freeformCustomToolBridgeStream) Close() error {
	return s.source.Close()
}
