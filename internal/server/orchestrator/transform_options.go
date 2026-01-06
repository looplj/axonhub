package orchestrator

import (
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

	if !transformOptions.ForceArrayInstructions && !transformOptions.ForceArrayInputs {
		return req
	}

	newReq := *req

	if transformOptions.ForceArrayInstructions {
		newReq.TransformOptions.ArrayInstructions = lo.ToPtr(true)
	}

	if transformOptions.ForceArrayInputs {
		newReq.TransformOptions.ArrayInputs = lo.ToPtr(true)
	}

	return &newReq
}
