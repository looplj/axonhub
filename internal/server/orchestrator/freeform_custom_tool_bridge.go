package orchestrator

import (
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// freeformCustomToolBridge is the orchestrator handle for a shared lifecycle
// bridge state. Conversion and rehydrate live in llm/transformer/shared.
type freeformCustomToolBridge = shared.FreeformCustomToolBridge

func candidateCanCarryOpenAICustomTools(candidate *ChannelModelsCandidate, req *llm.Request) bool {
	if candidateSupportsOpenAICustomTools(candidate) {
		return true
	}
	if candidate == nil || req == nil || !isResponsesFormat(req.APIFormat) {
		return false
	}

	switch llm.APIFormat(candidate.APIFormat) {
	case llm.APIFormatOpenAIChatCompletion, llm.APIFormatAnthropicMessage:
		return true
	default:
		return false
	}
}

func shouldBridgeOpenAICustomTools(candidate *ChannelModelsCandidate, req *llm.Request, outboundFormat llm.APIFormat) bool {
	if candidateSupportsOpenAICustomTools(candidate) {
		return false
	}
	if req == nil || !isResponsesFormat(req.APIFormat) {
		return false
	}

	switch outboundFormat {
	case llm.APIFormatOpenAIChatCompletion, llm.APIFormatAnthropicMessage:
		return true
	default:
		return false
	}
}

func bridgeOpenAICustomToolsToFunctions(
	req *llm.Request,
	targetFormat llm.APIFormat,
) (*llm.Request, *freeformCustomToolBridge, error) {
	return shared.BridgeOpenAICustomToolsToFunctions(req, targetFormat)
}

func newFreeformCustomToolBridgeStream(
	source streams.Stream[*llm.Response],
	bridge *freeformCustomToolBridge,
) streams.Stream[*llm.Response] {
	return shared.NewFreeformCustomToolBridgeStream(source, bridge)
}
