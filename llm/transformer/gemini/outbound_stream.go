package gemini

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// ErrMissingResponseID is returned when a meaningful chunk has no responseID
// and no real responseID has been received from previous chunks.
var ErrMissingResponseID = errors.New("missing responseID in stream: expected responseID after first event")

// defaultPendingResponseID is used when the first chunk has content but no responseID.
// This provides a temporary ID until a real responseID arrives.
const defaultPendingResponseID = "pending-response"

// streamState tracks state across streaming events.
type streamState struct {
	toolCallIndex          int
	responseID             string // Track responseID from chunks
	hasSeenMeaningfulEvent bool   // Track if we've processed at least one meaningful event
}

// TransformStream transforms the HTTP stream response to the unified response format.
// Gemini's stream is a stream of GenerateContentResponse.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	stream = streams.AppendStream(stream, lo.ToPtr(llm.DoneStreamEvent))

	// Track state across stream events
	state := &streamState{}

	mapped := streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.transformStreamChunkWithState(event, state)
	})

	// Filter out nil responses to ensure stream only returns valid responses
	return streams.NoNil(mapped), nil
}

// TransformStreamChunk transforms a single Gemini streaming chunk to unified Response.
// Note: This method does not track tool call index across events. Use TransformStream for proper streaming.
func (t *OutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	return t.transformStreamChunkWithState(event, &streamState{})
}

// transformStreamChunkWithState transforms a single Gemini streaming chunk with state tracking.
func (t *OutboundTransformer) transformStreamChunkWithState(
	event *httpclient.StreamEvent,
	state *streamState,
) (*llm.Response, error) {
	if event == nil || len(event.Data) == 0 {
		return nil, nil
	}

	// Handle [DONE] marker - Gemini doesn't use this, but handle it for consistency
	if string(event.Data) == "[DONE]" {
		return llm.DoneResponse, nil
	}

	// Parse the Gemini response chunk
	var resp GenerateContentResponse
	if err := json.Unmarshal(event.Data, &resp); err != nil {
		return nil, err
	}

	// Update state.responseID if chunk has a real responseID
	if resp.ResponseID != "" {
		state.responseID = resp.ResponseID
	}

	// Skip chunks with no meaningful content.
	// A chunk is meaningful if it has candidates OR usage metadata.
	// Intermediate chunks during thinking mode may have neither and can be safely skipped.
	if len(resp.Candidates) == 0 && resp.UsageMetadata == nil {
		return nil, nil
	}

	// This is a meaningful event - validate responseID requirements
	if state.hasSeenMeaningfulEvent {
		// Not the first event - error if current chunk has no ID and we're still using pending ID
		if resp.ResponseID == "" && state.responseID == defaultPendingResponseID {
			return nil, ErrMissingResponseID
		}
	}
	state.hasSeenMeaningfulEvent = true

	// Assign responseID if missing from this chunk
	if resp.ResponseID == "" {
		if state.responseID == "" {
			// First event without responseID - use pending
			state.responseID = defaultPendingResponseID
		}
		resp.ResponseID = state.responseID
	}

	// Convert to unified response format (streaming) with tool call index tracking
	llmResp, nextIndex := convertGeminiToLLMResponseWithState(&resp, true, state.toolCallIndex)
	state.toolCallIndex = nextIndex

	return llmResp, nil
}

// AggregateStreamChunks aggregates Gemini streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
