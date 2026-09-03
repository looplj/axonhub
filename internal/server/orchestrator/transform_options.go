package orchestrator

import (
	"maps"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
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

// applyReasoningEffortMapping applies the channel's reasoning effort mapping to the
// unified request before the outbound transformer runs, so the mapping affects every
// outbound protocol (chat completions, responses, messages) uniformly, regardless of
// which client the request came from.
func applyReasoningEffortMapping(req *llm.Request, channelSettings *objects.ChannelSettings) *llm.Request {
	if req == nil || channelSettings == nil {
		return req
	}

	mapped := llm.ApplyReasoningEffortMapping(req.ReasoningEffort, channelSettings.TransformOptions.ReasoningEffortMapping)
	if mapped == req.ReasoningEffort {
		return req
	}

	newReq := *req

	// The Anthropic inbound records the client's native output_config.effort in
	// TransformerMetadata and the outbound transformer rebuilds the upstream request
	// from that marker, so it must follow the mapped value or the mapping would never
	// reach messages-protocol channels. Clone the map first: the shallow request copy
	// still shares it with the original request.
	if _, ok := newReq.TransformerMetadata[anthropic.TransformerMetadataKeyOutputConfigEffort]; ok {
		metadata := make(map[string]any, len(newReq.TransformerMetadata))
		maps.Copy(metadata, newReq.TransformerMetadata)

		metadata[anthropic.TransformerMetadataKeyOutputConfigEffort] = mapped
		newReq.TransformerMetadata = metadata
	}

	newReq.ReasoningEffort = mapped

	return &newReq
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
