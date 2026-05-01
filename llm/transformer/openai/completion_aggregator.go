package openai

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func AggregateCompletionStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		data, err := json.Marshal(&llm.Response{})
		return data, llm.ResponseMeta{}, err
	}

	var (
		lastChunkResponse *CompletionResponse
		id                string
		model             string
		created           int64
		usage             *llm.Usage
		accumulatedText   string
		finishReason      *string
	)

	for _, chunk := range chunks {
		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			continue
		}

		var compResp CompletionResponse
		if err := json.Unmarshal(chunk.Data, &compResp); err != nil {
			continue
		}

		lastChunkResponse = &compResp

		if id == "" && compResp.ID != "" {
			id = compResp.ID
		}
		if model == "" && compResp.Model != "" {
			model = compResp.Model
		}
		if created == 0 && compResp.Created != 0 {
			created = compResp.Created
		}

		for _, choice := range compResp.Choices {
			accumulatedText += choice.Text
			if choice.FinishReason != nil {
				finishReason = choice.FinishReason
			}
		}

		if compResp.Usage.PromptTokens > 0 || compResp.Usage.TotalTokens > 0 {
			usage = &llm.Usage{
				PromptTokens:     compResp.Usage.PromptTokens,
				CompletionTokens: compResp.Usage.CompletionTokens,
				TotalTokens:      compResp.Usage.TotalTokens,
			}
		}
	}

	if lastChunkResponse == nil {
		data, err := json.Marshal(&llm.Response{})
		return data, llm.ResponseMeta{}, err
	}

	if finishReason == nil {
		finishReason = lo.ToPtr("stop")
	}

	response := &llm.Response{
		ID:      id,
		Model:   model,
		Object:  "text_completion",
		Created: created,
		Completion: &llm.CompletionResponse{
			Choices: []llm.CompletionChoice{
				{
					Text:         accumulatedText,
					Index:        0,
					FinishReason: finishReason,
				},
			},
		},
		Usage:       usage,
		RequestType: llm.RequestTypeCompletion,
		APIFormat:   llm.APIFormatOpenAICompletion,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, llm.ResponseMeta{}, err
	}

	return data, llm.ResponseMeta{
		ID:    id,
		Usage: usage,
	}, nil
}
