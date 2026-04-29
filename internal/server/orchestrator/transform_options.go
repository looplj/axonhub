package orchestrator

import (
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
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

	// Track only input-shape changes; instruction array coercion does not invalidate raw input items.
	inputDirty := false

	if transformOptions.ForceArrayInputs {
		newReq.TransformOptions.ArrayInputs = lo.ToPtr(true)
		inputDirty = true
	}

	if transformOptions.ReplaceDeveloperRoleWithSystem {
		var replaced bool
		newReq.Messages, replaced = replaceDeveloperRoleWithSystemChanged(newReq.Messages)
		if replaced {
			inputDirty = true
		}
	}

	if inputDirty && isResponsesFormat(newReq.APIFormat) {
		// Channel candidates share the original request, so detach extensions before marking this variant dirty.
		newReq.ProtocolExtensions = llm.CloneProtocolExtensions(req.ProtocolExtensions)
		llm.MarkOpenAIResponsesInputDirty(&newReq)
	}

	return &newReq
}

// replaceDeveloperRoleWithSystem replaces developer role with system in messages.
func replaceDeveloperRoleWithSystem(messages []llm.Message) []llm.Message {
	result, _ := replaceDeveloperRoleWithSystemChanged(messages)
	return result
}

func replaceDeveloperRoleWithSystemChanged(messages []llm.Message) ([]llm.Message, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	replaced := false

	result := make([]llm.Message, len(messages))
	for i, msg := range messages {
		if strings.EqualFold(msg.Role, "developer") {
			msg.Role = "system"
			// The raw Responses item still says role=developer, so this message must be rebuilt.
			msg.ProtocolExtensions = nil
			replaced = true
		}

		result[i] = msg
	}

	if !replaced {
		return messages, false
	}

	return result, true
}
