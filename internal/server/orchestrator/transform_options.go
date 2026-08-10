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
