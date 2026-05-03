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
	result := applyTransformOptionsWithResult(req, channelSettings)

	return result.Request
}

type transformOptionsResult struct {
	Request *llm.Request
	Changed bool
}

func applyTransformOptionsWithResult(req *llm.Request, channelSettings *objects.ChannelSettings) transformOptionsResult {
	if channelSettings == nil {
		return transformOptionsResult{Request: req}
	}

	transformOptions := channelSettings.TransformOptions

	if !transformOptions.ForceArrayInstructions &&
		!transformOptions.ForceArrayInputs &&
		!transformOptions.ReplaceDeveloperRoleWithSystem {
		return transformOptionsResult{Request: req}
	}

	newReq := *req
	changed := false

	if transformOptions.ForceArrayInstructions {
		newReq.TransformOptions.ArrayInstructions = lo.ToPtr(true)
		changed = true
	}

	if transformOptions.ForceArrayInputs {
		newReq.TransformOptions.ArrayInputs = lo.ToPtr(true)
		changed = true
	}

	if transformOptions.ReplaceDeveloperRoleWithSystem {
		replacedMessages := replaceDeveloperRoleWithSystem(newReq.Messages)
		if len(replacedMessages) > 0 && &replacedMessages[0] != &newReq.Messages[0] {
			changed = true
		}
		newReq.Messages = replacedMessages
	}

	return transformOptionsResult{Request: &newReq, Changed: changed}
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
