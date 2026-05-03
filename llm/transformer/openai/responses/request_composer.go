package responses

import (
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

type requestComposer struct {
	req   *llm.Request
	scope shared.TransportScope
}

func newRequestComposer(req *llm.Request, scope shared.TransportScope) *requestComposer {
	return &requestComposer{
		req:   req,
		scope: scope,
	}
}

func (c *requestComposer) Compose() (Request, []byte, error) {
	if c == nil || c.req == nil {
		return Request{}, nil, fmt.Errorf("chat request is nil")
	}

	payload := c.structuredRequest()
	rawFields := c.rawEnvelopeFields()
	structuredFields, err := c.structuredFields(payload)
	if err != nil {
		return Request{}, nil, err
	}

	body, err := mergeRawObjects(rawFields, structuredFields)
	if err != nil {
		return Request{}, nil, err
	}

	return payload, body, nil
}

func (c *requestComposer) structuredRequest() Request {
	req := c.req
	tools := c.structuredTools()

	payload := Request{
		Model:                req.Model,
		Input:                convertInputFromMessages(req.Messages, req.TransformOptions, c.scope),
		Instructions:         convertInstructionsFromMessages(req.Messages),
		Tools:                tools,
		ParallelToolCalls:    req.ParallelToolCalls,
		Stream:               req.Stream,
		Text:                 convertToTextOptions(req),
		Store:                req.Store,
		ServiceTier:          req.ServiceTier,
		SafetyIdentifier:     req.SafetyIdentifier,
		User:                 req.User,
		Metadata:             req.Metadata,
		MaxOutputTokens:      req.MaxCompletionTokens,
		TopLogprobs:          req.TopLogprobs,
		TopP:                 req.TopP,
		ToolChoice:           convertToolChoice(req.ToolChoice),
		StreamOptions:        convertStreamOptions(req.StreamOptions, req.TransformerMetadata),
		Reasoning:            convertReasoning(req),
		PromptCacheKey:       req.PromptCacheKey,
		PreviousResponseID:   req.PreviousResponseID,
		Include:              xmap.GetStringSlice(req.TransformerMetadata, "include"),
		MaxToolCalls:         xmap.GetInt64Ptr(req.TransformerMetadata, "max_tool_calls"),
		PromptCacheRetention: xmap.GetStringPtr(req.TransformerMetadata, "prompt_cache_retention"),
		Truncation:           xmap.GetStringPtr(req.TransformerMetadata, "truncation"),
	}

	if len(payload.Tools) == 0 {
		payload.ParallelToolCalls = nil
	}

	if payload.MaxOutputTokens == nil {
		payload.MaxOutputTokens = req.MaxTokens
	}

	return payload
}

func (c *requestComposer) structuredTools() []Tool {
	req := c.req
	if req.TransformerMetadata == nil {
		req.TransformerMetadata = map[string]any{}
	}

	tools := make([]Tool, 0, len(req.Tools))
	for _, item := range req.Tools {
		switch item.Type {
		case llm.ToolTypeImageGeneration:
			tool := convertImageGenerationToTool(item)
			tools = append(tools, tool)
			req.TransformerMetadata["image_output_format"] = tool.OutputFormat
		case llm.ToolTypeResponsesCustomTool:
			tools = append(tools, convertCustomToTool(item))
		case "function":
			tools = append(tools, convertFunctionToTool(item))
		}
	}

	return tools
}

func (c *requestComposer) rawEnvelopeFields() map[string]json.RawMessage {
	requestExt := c.openAIResponsesRequestExtensions()
	if requestExt == nil {
		return nil
	}

	raw := map[string]json.RawMessage{}
	for key, value := range requestExt.TopLevelExtra {
		raw[key] = cloneRaw(value)
	}

	if c.canReplayTopLevelSemanticExtra() {
		for key, value := range requestExt.TopLevelSemanticExtra {
			raw[key] = cloneRaw(value)
		}
	}

	if len(raw) == 0 {
		return nil
	}

	return raw
}

func (c *requestComposer) structuredFields(payload Request) (map[string]json.RawMessage, error) {
	fields := map[string]json.RawMessage{}

	if err := setStructuredField(fields, "model", payload.Model); err != nil {
		return nil, err
	}
	if err := setStructuredField(fields, "instructions", payload.Instructions); err != nil {
		return nil, err
	}

	inputRaw, err := c.inputField(payload)
	if err != nil {
		return nil, err
	}
	fields["input"] = inputRaw

	toolsRaw, err := c.toolsField(payload)
	if err != nil {
		return nil, err
	}
	if len(toolsRaw) > 0 {
		fields["tools"] = toolsRaw
	}

	if payload.ParallelToolCalls != nil {
		if err := setStructuredField(fields, "parallel_tool_calls", payload.ParallelToolCalls); err != nil {
			return nil, err
		}
	}
	if payload.Background != nil {
		if err := setStructuredField(fields, "background", payload.Background); err != nil {
			return nil, err
		}
	}
	if payload.Stream != nil {
		if err := setStructuredField(fields, "stream", payload.Stream); err != nil {
			return nil, err
		}
	}
	if payload.Store != nil {
		if err := setStructuredField(fields, "store", payload.Store); err != nil {
			return nil, err
		}
	}
	if payload.ServiceTier != nil {
		if err := setStructuredField(fields, "service_tier", payload.ServiceTier); err != nil {
			return nil, err
		}
	}
	if payload.SafetyIdentifier != nil {
		if err := setStructuredField(fields, "safety_identifier", payload.SafetyIdentifier); err != nil {
			return nil, err
		}
	}
	if payload.User != nil {
		if err := setStructuredField(fields, "user", payload.User); err != nil {
			return nil, err
		}
	}
	if metadata := c.metadataField(payload); len(metadata) > 0 {
		fields["metadata"] = metadata
	}
	if payload.MaxOutputTokens != nil {
		if err := setStructuredField(fields, "max_output_tokens", payload.MaxOutputTokens); err != nil {
			return nil, err
		}
	}
	if payload.MaxToolCalls != nil {
		if err := setStructuredField(fields, "max_tool_calls", payload.MaxToolCalls); err != nil {
			return nil, err
		}
	}
	if payload.Text != nil {
		if err := setStructuredField(fields, "text", payload.Text); err != nil {
			return nil, err
		}
	}
	if len(payload.Include) > 0 {
		if err := setStructuredField(fields, "include", payload.Include); err != nil {
			return nil, err
		}
	}
	if payload.PreviousResponseID != nil {
		if err := setStructuredField(fields, "previous_response_id", payload.PreviousResponseID); err != nil {
			return nil, err
		}
	}
	if payload.PromptCacheKey != nil {
		if err := setStructuredField(fields, "prompt_cache_key", payload.PromptCacheKey); err != nil {
			return nil, err
		}
	}
	if payload.PromptCacheRetention != nil {
		if err := setStructuredField(fields, "prompt_cache_retention", payload.PromptCacheRetention); err != nil {
			return nil, err
		}
	}
	if payload.Reasoning != nil {
		if err := setStructuredField(fields, "reasoning", payload.Reasoning); err != nil {
			return nil, err
		}
	}
	if payload.StreamOptions != nil {
		if err := setStructuredField(fields, "stream_options", payload.StreamOptions); err != nil {
			return nil, err
		}
	}
	if toolChoice := c.toolChoiceField(payload); len(toolChoice) > 0 {
		fields["tool_choice"] = toolChoice
	}
	if payload.Truncation != nil {
		if err := setStructuredField(fields, "truncation", payload.Truncation); err != nil {
			return nil, err
		}
	}
	if payload.TopLogprobs != nil {
		if err := setStructuredField(fields, "top_logprobs", payload.TopLogprobs); err != nil {
			return nil, err
		}
	}
	if payload.TopP != nil {
		if err := setStructuredField(fields, "top_p", payload.TopP); err != nil {
			return nil, err
		}
	}

	return fields, nil
}

func (c *requestComposer) inputField(payload Request) (json.RawMessage, error) {
	requestExt := c.openAIResponsesRequestExtensions()
	if requestExt != nil && c.canReplayRawInput() && len(requestExt.InputRaw) > 0 {
		return cloneRaw(requestExt.InputRaw), nil
	}

	return marshalRaw(payload.Input)
}

func (c *requestComposer) toolsField(payload Request) (json.RawMessage, error) {
	requestExt := c.openAIResponsesRequestExtensions()
	if requestExt != nil && c.canReplayRawTools() {
		rawTools := rawItemsArray(requestExt.Tools)
		if len(rawTools) > 0 {
			return rawTools, nil
		}
	}

	if len(payload.Tools) == 0 {
		return nil, nil
	}

	return marshalRaw(payload.Tools)
}

func (c *requestComposer) toolChoiceField(payload Request) json.RawMessage {
	requestExt := c.openAIResponsesRequestExtensions()
	if requestExt != nil && c.canReplayRawToolChoice() && len(requestExt.ToolChoiceRaw) > 0 {
		return cloneRaw(requestExt.ToolChoiceRaw)
	}

	if payload.ToolChoice == nil {
		return nil
	}

	raw, err := marshalRaw(payload.ToolChoice)
	if err != nil {
		return nil
	}

	return raw
}

func (c *requestComposer) metadataField(payload Request) json.RawMessage {
	requestExt := c.openAIResponsesRequestExtensions()
	if requestExt != nil && len(requestExt.MetadataRaw) > 0 {
		return cloneRaw(requestExt.MetadataRaw)
	}

	if len(payload.Metadata) == 0 {
		return nil
	}

	raw, err := marshalRaw(payload.Metadata)
	if err != nil {
		return nil
	}

	return raw
}

func (c *requestComposer) openAIResponsesRequestExtensions() *llm.OpenAIResponsesRequestExtensions {
	if c == nil || c.req == nil || c.req.ProviderExtensions == nil ||
		c.req.ProviderExtensions.OpenAIResponses == nil {
		return nil
	}

	return c.req.ProviderExtensions.OpenAIResponses.Request
}

func (c *requestComposer) dirty() llm.OpenAIResponsesDirtySet {
	if c == nil || c.req == nil || c.req.ProviderExtensions == nil ||
		c.req.ProviderExtensions.OpenAIResponses == nil {
		return llm.OpenAIResponsesDirtySet{}
	}

	return c.req.ProviderExtensions.OpenAIResponses.Dirty
}

func (c *requestComposer) canReplayTopLevelSemanticExtra() bool {
	dirty := c.dirty()

	return !dirty.HasAny(
		llm.OpenAIResponsesDirtyMessages,
		llm.OpenAIResponsesDirtyInstructions,
		llm.OpenAIResponsesDirtyInputItems,
		llm.OpenAIResponsesDirtyTopLevelSemanticExtra,
	)
}

func (c *requestComposer) canReplayRawInput() bool {
	dirty := c.dirty()
	if dirty.HasAny(llm.OpenAIResponsesDirtyMessages, llm.OpenAIResponsesDirtyInputItems) {
		return false
	}

	requestExt := c.openAIResponsesRequestExtensions()
	if requestExt == nil {
		return false
	}

	for _, item := range requestExt.InputItems {
		if item.SemanticKey != "" {
			continue
		}
		if len(item.Raw) == 0 {
			continue
		}
		if !item.Protection.ReplayAllowed {
			return false
		}
	}

	return true
}

func (c *requestComposer) canReplayRawTools() bool {
	return !c.dirty().Has(llm.OpenAIResponsesDirtyTools)
}

func (c *requestComposer) canReplayRawToolChoice() bool {
	return !c.dirty().Has(llm.OpenAIResponsesDirtyToolChoice)
}

func rawItemsArray(items []llm.OpenAIResponsesRawItem) json.RawMessage {
	if len(items) == 0 {
		return nil
	}

	rawItems := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		if len(item.Raw) == 0 {
			return nil
		}
		rawItems = append(rawItems, cloneRaw(item.Raw))
	}

	raw, err := json.Marshal(rawItems)
	if err != nil {
		return nil
	}

	return raw
}

func setStructuredField(fields map[string]json.RawMessage, key string, value any) error {
	raw, err := marshalRaw(value)
	if err != nil {
		return fmt.Errorf("marshal responses request field %s: %w", key, err)
	}

	fields[key] = raw

	return nil
}
