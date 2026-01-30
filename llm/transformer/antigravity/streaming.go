package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// StreamProcessor handles Antigravity SSE streaming.
type StreamProcessor struct {
	reader *bufio.Reader
	// we use a gemini transformer to convert chunks
	geminiTransformer transformer.Outbound
}

func NewStreamProcessor(reader io.Reader, geminiTransformer transformer.Outbound) *StreamProcessor {
	return &StreamProcessor{
		reader:            bufio.NewReader(reader),
		geminiTransformer: geminiTransformer,
	}
}

// Recv returns the next response chunk.
func (p *StreamProcessor) Recv(ctx context.Context) (*llm.Response, error) {
	for {
		line, err := p.reader.ReadBytes('\n')
		handleEOF := false
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				// We have a partial line (final chunk without newline) - process it
				handleEOF = true
			} else {
				// Either EOF with no data, or a different error
				return nil, err
			}
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			if handleEOF {
				return nil, io.EOF
			}
			continue
		}

		if !bytes.HasPrefix(line, []byte("data:")) {
			if handleEOF {
				return nil, io.EOF
			}
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data:"))
		data = bytes.TrimSpace(data)

		if string(data) == "[DONE]" {
			return nil, io.EOF
		}

		// Hack: use the gemini transformer to convert the chunk.
		// We create a fake http response with the chunk body.
		// Unwrapping: Extract the inner `response` object.

		var wrapper struct {
			Response json.RawMessage `json:"response"`
		}

		// If it unmarshals into wrapper with response field, we use that.
		// If not (maybe it's direct gemini response), we try directly.
		var chunkBytes []byte
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Response) > 0 {
			chunkBytes = wrapper.Response
		} else {
			chunkBytes = data
		}

		// Wrap chunk in fake HTTP response for Gemini transformer
		fakeResp := &httpclient.Response{
			StatusCode: 200,
			Body:       chunkBytes,
		}

		// TransformResponse uses isStream=false, so we'll fix up the response below.
		llmResp, err := p.geminiTransformer.TransformResponse(ctx, fakeResp)
		if err != nil {
			return nil, fmt.Errorf("gemini transform error: %w", err)
		}

		// Fix up for streaming: Move Message to Delta
		if len(llmResp.Choices) > 0 {
			for i := range llmResp.Choices {
				if llmResp.Choices[i].Message != nil {
					llmResp.Choices[i].Delta = llmResp.Choices[i].Message
					llmResp.Choices[i].Message = nil
				}
			}
		}
		// Also object type
		llmResp.Object = "chat.completion.chunk"

		return llmResp, nil
	}
}
