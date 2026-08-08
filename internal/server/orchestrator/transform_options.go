package orchestrator

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// applyTransformOptions applies channel transform options to create a new llm.Request.
// It creates a new request instead of modifying the original one.
func applyTransformOptions(req *llm.Request, channelSettings *objects.ChannelSettings) *llm.Request {
	if channelSettings == nil {
		return req
	}

	transformOptions := channelSettings.TransformOptions

	if !transformOptions.ForceArrayInstructions &&
		!transformOptions.ForceArrayInputs &&
		!transformOptions.ReplaceDeveloperRoleWithSystem {
		return req
	}

	newReq := *req

	if transformOptions.ForceArrayInstructions {
		newReq.TransformOptions.ArrayInstructions = lo.ToPtr(true)
	}

	if transformOptions.ForceArrayInputs {
		newReq.TransformOptions.ArrayInputs = lo.ToPtr(true)
	}

	if transformOptions.ReplaceDeveloperRoleWithSystem {
		newReq.Messages = replaceDeveloperRoleWithSystem(newReq.Messages)
	}

	return &newReq
}

// applyClaudeCodeCacheCompatibility keeps the stable prompt prefix produced by Claude Code.
// Claude Code can inject real system-role messages after the leading system block. OpenAI
// Responses and Anthropic Messages move those messages into their leading instruction/system
// fields, and many OpenAI-compatible Chat Completions upstreams do the same internally. That
// changes the beginning of the prompt on later turns and defeats prefix caching.
//
// Only Claude Code requests targeting an affected text protocol are considered. Existing user
// messages that merely contain <system-reminder> text are already safe and remain untouched.
func applyClaudeCodeCacheCompatibility(req *llm.Request, outboundFormat llm.APIFormat) *llm.Request {
	if !isClaudeCodeRequest(req) || !supportsClaudeCodeSystemDowngrade(outboundFormat) {
		return req
	}

	messages, changed := downgradeMidConversationSystemMessages(req.Messages)
	if !changed {
		return req
	}

	newReq := *req
	newReq.Messages = messages

	return &newReq
}

func isClaudeCodeRequest(req *llm.Request) bool {
	if req == nil || req.RawRequest == nil || req.RawRequest.Headers == nil {
		return false
	}

	userAgent := strings.TrimSpace(req.RawRequest.Headers.Get("User-Agent"))

	return strings.HasPrefix(userAgent, "claude-cli/")
}

func supportsClaudeCodeSystemDowngrade(format llm.APIFormat) bool {
	//nolint:exhaustive // Only text protocols that may hoist system messages are relevant.
	switch format {
	case llm.APIFormatOpenAIChatCompletion,
		llm.APIFormatOpenAIResponse,
		llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatAnthropicMessage:
		return true
	default:
		return false
	}
}

// applyClaudeCodeOpenAIReasoningEffortMapping is a common fallback for dedicated
// OpenAI-compatible transformers that do not consume the channel mapping themselves.
// Existing transformer-level mapping remains authoritative: if the final outbound value
// already differs from the original unified value, this function leaves it unchanged.
func applyClaudeCodeOpenAIReasoningEffortMapping(
	httpRequest *httpclient.Request,
	channelSettings *objects.ChannelSettings,
	outboundFormat llm.APIFormat,
	isClaudeCodeClient bool,
	originalReasoningEffort string,
) (*httpclient.Request, error) {
	if httpRequest == nil || channelSettings == nil || !isClaudeCodeClient || originalReasoningEffort == "" {
		return httpRequest, nil
	}

	mappings := channelSettings.TransformOptions.ReasoningEffortMapping
	if len(mappings) == 0 {
		return httpRequest, nil
	}

	path := ""
	//nolint:exhaustive // Only OpenAI text protocols expose a reasoning effort field.
	switch outboundFormat {
	case llm.APIFormatOpenAIChatCompletion:
		path = "reasoning_effort"
	case llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact:
		path = "reasoning.effort"
	default:
		return httpRequest, nil
	}

	finalEffort := gjson.GetBytes(httpRequest.Body, path)
	if finalEffort.Type != gjson.String || finalEffort.String() != originalReasoningEffort {
		return httpRequest, nil
	}

	mappedEffort := originalReasoningEffort
	for _, mapping := range mappings {
		if mapping.From == originalReasoningEffort {
			mappedEffort = mapping.To
			break
		}
	}
	if mappedEffort == originalReasoningEffort {
		return httpRequest, nil
	}

	body, err := sjson.SetBytes(httpRequest.Body, path, mappedEffort)
	if err != nil {
		return nil, fmt.Errorf("failed to apply Claude Code reasoning effort mapping: %w", err)
	}

	cloned := *httpRequest
	cloned.Body = body

	if jsonEffort := gjson.GetBytes(httpRequest.JSONBody, path); jsonEffort.Type == gjson.String && jsonEffort.String() == originalReasoningEffort {
		cloned.JSONBody, err = sjson.SetBytes(httpRequest.JSONBody, path, mappedEffort)
		if err != nil {
			return nil, fmt.Errorf("failed to apply Claude Code reasoning effort mapping to log body: %w", err)
		}
	}

	return &cloned, nil
}

// downgradeMidConversationSystemMessages preserves the leading contiguous system block and
// changes every later system role to user. All later system messages must be handled together:
// a trailing reminder becomes part of the history on the next turn, so changing only the final
// message would make its role alternate between turns and destabilize the cache again.
func downgradeMidConversationSystemMessages(messages []llm.Message) ([]llm.Message, bool) {
	leadingSystemEnd := 0
	for leadingSystemEnd < len(messages) && strings.EqualFold(messages[leadingSystemEnd].Role, "system") {
		leadingSystemEnd++
	}

	var result []llm.Message
	for i := leadingSystemEnd; i < len(messages); i++ {
		if !strings.EqualFold(messages[i].Role, "system") {
			continue
		}

		if result == nil {
			result = append([]llm.Message(nil), messages...)
		}
		result[i].Role = "user"
	}

	if result == nil {
		return messages, false
	}

	return result, true
}

// replaceDeveloperRoleWithSystem replaces developer role with system in messages.
func replaceDeveloperRoleWithSystem(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}

	replaced := false

	result := make([]llm.Message, len(messages))
	for i, msg := range messages {
		if strings.EqualFold(msg.Role, "developer") {
			msg.Role = "system"
			replaced = true
		}

		result[i] = msg
	}

	if !replaced {
		return messages
	}

	return result
}
