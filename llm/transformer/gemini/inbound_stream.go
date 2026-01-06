package gemini

import (
	"context"
	"encoding/json"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// TransformStream transforms the unified stream response format to Gemini HTTP response stream.
// Gemini's stream format is a stream of GenerateContentResponse.
func (t *InboundTransformer) TransformStream(
	ctx context.Context,
	llmStream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	stream := streams.MapErr(llmStream, func(chunk *llm.Response) (*httpclient.StreamEvent, error) {
		return t.TransformStreamChunk(ctx, chunk)
	})

	return streams.NoNil(stream), nil
}

// TransformStreamChunk transforms a single unified Response chunk to Gemini StreamEvent.
func (t *InboundTransformer) TransformStreamChunk(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.StreamEvent, error) {
	if chatResp == nil {
		return nil, nil
	}

	// Handle [DONE] marker
	if chatResp.Object == "[DONE]" {
		// Gemini doesn't use [DONE] marker, but we can return an nil to signal the end of the stream.
		//nolint:nilnil // Checked.
		return nil, nil
	}

	// Convert to Gemini response format (streaming)
	geminiResp := convertLLMToGeminiResponse(chatResp, true)

	eventData, err := json.Marshal(geminiResp)
	if err != nil {
		return nil, err
	}

	return &httpclient.StreamEvent{
		Data: eventData,
	}, nil
}

// AggregateStreamChunks aggregates streaming chunks into a complete response body in Gemini format.
func (t *InboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
