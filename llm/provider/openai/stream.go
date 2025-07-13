package openai

import (
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"

	"github.com/looplj/axonhub/llm/types"
)

type streamAdapter struct {
	ChatCompletionStream *openai.ChatCompletionStream
	current              *types.ChatCompletionResponse
	err                  error
}

func (s *streamAdapter) Next() bool {
	resp, err := s.ChatCompletionStream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false
		}
		s.err = err
		return false
	}
	s.current, s.err = convertFromOpenAIStreamResponse(&resp)
	return s.err == nil
}

func (s *streamAdapter) Current() *types.ChatCompletionResponse {
	return s.current
}

func (s *streamAdapter) Err() error {
	return s.err
}

func (s *streamAdapter) Close() error {
	return s.ChatCompletionStream.Close()
}
