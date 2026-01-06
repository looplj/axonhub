package gemini

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// candidateAggregator is a helper struct to aggregate data for each candidate.
type candidateAggregator struct {
	index             int64
	textParts         strings.Builder
	reasoningContent  strings.Builder
	toolCalls         map[int]*llm.ToolCall
	inlineDataParts   []*Blob
	finishReason      string
	groundingMetadata *GroundingMetadata
}

// AggregateStreamChunks aggregates Gemini streaming response chunks into a complete response.
// The output is always in Gemini format.
func AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		data, err := json.Marshal(&GenerateContentResponse{})
		return data, llm.ResponseMeta{}, err
	}

	var (
		lastResp      *GenerateContentResponse
		usage         *UsageMetadata
		responseID    string
		modelVersion  string
		candidateAggs = make(map[int64]*candidateAggregator)
	)

	for _, chunk := range chunks {
		// Skip empty or [DONE] events
		if chunk == nil || len(chunk.Data) == 0 || string(chunk.Data) == "[DONE]" {
			continue
		}

		var geminiResp GenerateContentResponse
		if err := json.Unmarshal(chunk.Data, &geminiResp); err != nil {
			continue // Skip invalid chunks
		}

		// Capture response ID and model version
		if responseID == "" && geminiResp.ResponseID != "" {
			responseID = geminiResp.ResponseID
		}

		if modelVersion == "" && geminiResp.ModelVersion != "" {
			modelVersion = geminiResp.ModelVersion
		}

		// Process each candidate
		for _, candidate := range geminiResp.Candidates {
			if candidate == nil {
				continue
			}

			candidateIndex := candidate.Index

			// Initialize candidate aggregator if it doesn't exist
			if _, ok := candidateAggs[candidateIndex]; !ok {
				candidateAggs[candidateIndex] = &candidateAggregator{
					index:     candidateIndex,
					toolCalls: make(map[int]*llm.ToolCall),
				}
			}

			agg := candidateAggs[candidateIndex]

			// Process content parts
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part == nil {
						continue
					}

					switch {
					case part.Text != "":
						if part.Thought {
							agg.reasoningContent.WriteString(part.Text)
						} else {
							agg.textParts.WriteString(part.Text)
						}

					case part.FunctionCall != nil:
						toolCallIndex := len(agg.toolCalls)
						argsJSON, _ := json.Marshal(part.FunctionCall.Args)
						agg.toolCalls[toolCallIndex] = &llm.ToolCall{
							Index: toolCallIndex,
							ID:    part.FunctionCall.ID,
							Type:  "function",
							Function: llm.FunctionCall{
								Name:      part.FunctionCall.Name,
								Arguments: string(argsJSON),
							},
						}

					case part.InlineData != nil:
						agg.inlineDataParts = append(agg.inlineDataParts, part.InlineData)
					}
				}
			}

			// Capture finish reason
			if candidate.FinishReason != "" {
				agg.finishReason = candidate.FinishReason
			}

			// Capture grounding metadata (use the last one if multiple chunks have it)
			if candidate.GroundingMetadata != nil {
				agg.groundingMetadata = candidate.GroundingMetadata
			}
		}

		// Extract usage information if present
		if geminiResp.UsageMetadata != nil {
			usage = geminiResp.UsageMetadata
		}

		lastResp = &geminiResp
	}

	// Build the final response in Gemini format
	if lastResp == nil {
		data, err := json.Marshal(&GenerateContentResponse{})
		return data, llm.ResponseMeta{}, err
	}

	return buildGeminiResponse(candidateAggs, responseID, modelVersion, usage)
}

// buildGeminiResponse builds a Gemini format response from aggregated data.
func buildGeminiResponse(
	candidateAggs map[int64]*candidateAggregator,
	responseID, modelVersion string,
	usage *UsageMetadata,
) ([]byte, llm.ResponseMeta, error) {
	candidates := make([]*Candidate, len(candidateAggs))

	for i := range candidates {
		agg := candidateAggs[int64(i)]
		if agg == nil {
			continue
		}

		content := &Content{
			Role: "model",
		}

		parts := make([]*Part, 0)

		// Add reasoning content (thinking) first if present
		if agg.reasoningContent.Len() > 0 {
			parts = append(parts, &Part{
				Text:    agg.reasoningContent.String(),
				Thought: true,
			})
		}

		// Add text content
		if agg.textParts.Len() > 0 {
			parts = append(parts, &Part{Text: agg.textParts.String()})
		}

		// Add tool calls
		for idx := range len(agg.toolCalls) {
			tc := agg.toolCalls[idx]

			var args map[string]any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			}

			parts = append(parts, &Part{
				FunctionCall: &FunctionCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: args,
				},
			})
		}

		// Add inline data parts (images, etc.)
		for _, inlineData := range agg.inlineDataParts {
			parts = append(parts, &Part{
				InlineData: inlineData,
			})
		}

		content.Parts = parts

		// Determine finish reason
		finishReason := agg.finishReason
		if finishReason == "" {
			finishReason = "STOP"
		}

		candidates[i] = &Candidate{
			Index:             agg.index,
			Content:           content,
			FinishReason:      finishReason,
			GroundingMetadata: agg.groundingMetadata,
		}
	}

	// Build the final response
	response := &GenerateContentResponse{
		ResponseID:    responseID,
		ModelVersion:  modelVersion,
		Candidates:    candidates,
		UsageMetadata: usage,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, llm.ResponseMeta{}, err
	}

	var llmUsage *llm.Usage
	if usage != nil {
		llmUsage = &llm.Usage{
			PromptTokens:     usage.PromptTokenCount,
			CompletionTokens: usage.CandidatesTokenCount,
			TotalTokens:      usage.TotalTokenCount,
		}
	}

	return data, llm.ResponseMeta{
		ID:    responseID,
		Usage: llmUsage,
	}, nil
}
