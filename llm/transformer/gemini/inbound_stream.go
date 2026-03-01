package gemini

import (
	"context"
	"encoding/json"
	"maps"
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/streams"
)

// TransformStream transforms the unified stream response format to Gemini HTTP response stream.
// Gemini's stream format is a stream of GenerateContentResponse.
func (t *InboundTransformer) TransformStream(
	ctx context.Context,
	llmStream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return &geminiInboundStream{
		ctx:                          ctx,
		source:                       llmStream,
		pendingToolCallsByC:          make(map[int]map[int]*toolCallAgg),
		pendingReasoningSignatureByC: make(map[int]*string),
	}, nil
}

type toolCallAgg struct {
	index               int
	id                  string
	typ                 string
	name                string
	arguments           string
	transformerMetadata map[string]any
}

// geminiInboundStream is a stateful transformer that aggregates tool call deltas.
// Gemini clients expect each streamed functionCall part to contain a complete args object,
// but OpenAI/Anthropic-compatible streams may send partial tool call deltas.
//
//nolint:containedctx // Checked.
type geminiInboundStream struct {
	ctx    context.Context
	source streams.Stream[*llm.Response]

	current *httpclient.StreamEvent
	err     error

	// choiceIndex -> toolCallIndex -> aggregator
	pendingToolCallsByC map[int]map[int]*toolCallAgg
	// choiceIndex -> last seen reasoning signature delta (may arrive as a standalone chunk)
	pendingReasoningSignatureByC map[int]*string
}

func (s *geminiInboundStream) Next() bool {
	if s.err != nil {
		return false
	}

	for s.source.Next() {
		chunk := s.source.Current()
		event, err := s.transformChunk(chunk)
		if err != nil {
			s.err = err
			return false
		}

		if event == nil {
			continue
		}

		s.current = event
		return true
	}

	if err := s.source.Err(); err != nil {
		s.err = err
	}

	return false
}

func (s *geminiInboundStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *geminiInboundStream) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.source.Err()
}

func (s *geminiInboundStream) Close() error {
	return s.source.Close()
}

func (s *geminiInboundStream) transformChunk(chunk *llm.Response) (*httpclient.StreamEvent, error) {
	if chunk == nil {
		return nil, nil
	}

	// Handle [DONE] marker
	if chunk.Object == "[DONE]" {
		return nil, nil
	}

	// Fast-path: no choices
	if len(chunk.Choices) == 0 {
		return (&InboundTransformer{}).TransformStreamChunk(s.ctx, chunk)
	}

	out := &llm.Response{
		ID:                chunk.ID,
		Object:            chunk.Object,
		Created:           chunk.Created,
		Model:             chunk.Model,
		SystemFingerprint: chunk.SystemFingerprint,
		ServiceTier:       chunk.ServiceTier,
		Usage:             chunk.Usage,
		Error:             chunk.Error,
		RequestType:       chunk.RequestType,
		TransformerMetadata: func() map[string]any {
			if chunk.TransformerMetadata == nil {
				return nil
			}
			return maps.Clone(chunk.TransformerMetadata)
		}(),
		Choices: make([]llm.Choice, 0, len(chunk.Choices)),
	}

	emitAny := false

	for _, choice := range chunk.Choices {
		outChoice := llm.Choice{
			Index:        choice.Index,
			FinishReason: choice.FinishReason,
			Logprobs:     choice.Logprobs,
			TransformerMetadata: func() map[string]any {
				if choice.TransformerMetadata == nil {
					return nil
				}
				return maps.Clone(choice.TransformerMetadata)
			}(),
		}

		srcMsg := choice.Delta
		targetIsDelta := true
		if srcMsg == nil {
			srcMsg = choice.Message
			targetIsDelta = false
		}

		if srcMsg == nil {
			out.Choices = append(out.Choices, outChoice)
			continue
		}

		// Copy message but strip tool calls; we will re-inject only completed ones.
		dstMsg := *srcMsg
		dstMsg.ToolCalls = nil

		choiceIndex := choice.Index
		if _, ok := s.pendingToolCallsByC[choiceIndex]; !ok {
			s.pendingToolCallsByC[choiceIndex] = make(map[int]*toolCallAgg)
		}
		if s.pendingReasoningSignatureByC == nil {
			s.pendingReasoningSignatureByC = make(map[int]*string)
		}
		// Buffer a signature-only delta for later: Anthropic streaming may deliver signature
		// in a standalone chunk (signature_delta) before/after tool call chunks.
		if dstMsg.ReasoningSignature != nil && *dstMsg.ReasoningSignature != "" {
			s.pendingReasoningSignatureByC[choiceIndex] = dstMsg.ReasoningSignature
		} else if pending := s.pendingReasoningSignatureByC[choiceIndex]; pending != nil && *pending != "" {
			// Carry the pending signature forward so conversion can attach it to a Gemini part.
			dstMsg.ReasoningSignature = pending
		}

		pendingByIndex := s.pendingToolCallsByC[choiceIndex]

		// Accumulate tool call deltas.
		for _, tc := range srcMsg.ToolCalls {
			idx := tc.Index
			agg, ok := pendingByIndex[idx]
			if !ok {
				agg = &toolCallAgg{index: idx}
				pendingByIndex[idx] = agg
			}

			if tc.ID != "" {
				agg.id = tc.ID
			}
			if tc.Type != "" {
				agg.typ = tc.Type
			}
			if tc.Function.Name != "" {
				agg.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				agg.arguments += tc.Function.Arguments
			}
			if tc.TransformerMetadata != nil {
				if agg.transformerMetadata == nil {
					agg.transformerMetadata = map[string]any{}
				}
				maps.Copy(agg.transformerMetadata, tc.TransformerMetadata)
			}
		}

		// Decide if we should flush tool calls:
		// - If any pending tool call has valid JSON args, flush that tool call.
		// - If finish_reason == "tool_calls", flush all pending tool calls (repairing JSON if needed).
		flushAll := choice.FinishReason != nil && *choice.FinishReason == "tool_calls"

		var completed []llm.ToolCall

		// Iterate tool calls in index order for deterministic output.
		keys := make([]int, 0, len(pendingByIndex))
		for idx := range pendingByIndex {
			keys = append(keys, idx)
		}
		sort.Ints(keys)

		for _, idx := range keys {
			agg := pendingByIndex[idx]
			if agg == nil {
				continue
			}

			args := agg.arguments
			// Gemini requires function_call.name, so don't emit tool calls until we have it.
			if agg.name == "" {
				// Even if flushAll, emitting an empty name would poison subsequent rounds (client will replay it).
				continue
			}

			// If arguments were never provided, don't emit early. We only know it's a no-arg tool call when the tool_calls turn ends.
			if strings.TrimSpace(args) == "" {
				if !flushAll {
					continue
				}
				args = "{}"
			}

			if flushAll && args != "" && !json.Valid([]byte(args)) {
				args = string(xjson.SafeJSONRawMessage(args))
			}

			isValidNow := json.Valid([]byte(args))
			if !flushAll && !isValidNow {
				continue
			}

			// When not flushing all, only emit once we have valid JSON.
			completed = append(completed, llm.ToolCall{
				ID:   agg.id,
				Type: agg.typ,
				Function: llm.FunctionCall{
					Name:      agg.name,
					Arguments: args,
				},
				Index:               agg.index,
				TransformerMetadata: agg.transformerMetadata,
			})

			delete(pendingByIndex, idx)
		}

		if len(completed) > 0 {
			dstMsg.ToolCalls = completed
		}

		if targetIsDelta {
			outChoice.Delta = &dstMsg
		} else {
			outChoice.Message = &dstMsg
		}

		// Emit if there is any non-tool content, any flushed tool call, or a finish reason.
		hasNonToolContent := (dstMsg.Content.Content != nil && *dstMsg.Content.Content != "") ||
			len(dstMsg.Content.MultipleContent) > 0 ||
			(dstMsg.ReasoningContent != nil && *dstMsg.ReasoningContent != "") ||
			dstMsg.Refusal != ""
		if hasNonToolContent || len(completed) > 0 || outChoice.FinishReason != nil {
			emitAny = true
			// Once we've emitted a chunk that can carry the signature, clear the pending buffer.
			// This avoids re-attaching the same signature onto subsequent chunks.
			if dstMsg.ReasoningSignature != nil && *dstMsg.ReasoningSignature != "" {
				delete(s.pendingReasoningSignatureByC, choiceIndex)
			}
		}

		out.Choices = append(out.Choices, outChoice)
	}

	if !emitAny {
		return nil, nil
	}

	geminiResp := convertLLMToGeminiResponse(out, true)
	eventData, err := json.Marshal(geminiResp)
	if err != nil {
		return nil, err
	}

	return &httpclient.StreamEvent{Data: eventData}, nil
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
