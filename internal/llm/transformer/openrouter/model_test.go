package openrouter_test

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer/openrouter"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestResponse_ToOpenAIResponse(t *testing.T) {
	tests := []struct {
		file string
		name string // description of this test case
		want *llm.Response
	}{
		{
			file: "or-chunk.json",
			name: "or-chunk",
			want: &llm.Response{
				ID:      "gen-1758295230-SiI5bLSgznz9dz6HO9XP",
				Model:   "z-ai/glm-4.5-air:free",
				Object:  "chat.completion.chunk",
				Created: 1758295230,
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr(""),
							},
							ReasoningContent: lo.ToPtr("We"),
						},
					},
				},
			},
		},
		{
			file: "or-image.json",
			name: "or-image",
			want: &llm.Response{
				ID:      "gen-1759393520-lxwGJP80UyDdG9VmVTQj",
				Model:   "google/gemini-2.5-flash-image-preview",
				Object:  "chat.completion",
				Created: 1759393520,
				Choices: []llm.Choice{
					{
						Index:        0,
						FinishReason: lo.ToPtr("stop"),
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "data:image/png;base64,iVBORw0KGgo",
										},
									},
								},
							},
						},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     7,
					CompletionTokens: 1290,
					TotalTokens:      1297,
					PromptTokensDetails: &llm.PromptTokensDetails{
						CachedTokens: 0,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 0,
					},
				},
			},
		},
		{
			file: "or-tool.json",
			name: "or-tool",
			want: &llm.Response{
				ID:      "gen-1761364094-YUYoOUHw4ouwIQdCevsI",
				Model:   "minimax/minimax-m2:free",
				Object:  "chat.completion",
				Created: 1761364095,
				Choices: []llm.Choice{
					{
						Index:        0,
						FinishReason: lo.ToPtr("tool_calls"),
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr(""),
							},
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_function_2653208710_1",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "Read",
										Arguments: "{\"file_path\": \"/axonhub/internal/server/middleware/trace_test.go\"}",
									},
								},
							},
						},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     15604,
					CompletionTokens: 71,
					TotalTokens:      15675,
					PromptTokensDetails: &llm.PromptTokensDetails{
						AudioTokens:  0,
						CachedTokens: 0,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						AudioTokens:              0,
						ReasoningTokens:          0,
						AcceptedPredictionTokens: 0,
						RejectedPredictionTokens: 0,
					},
				},
			},
		},

		{
			name: "or-tool-chunk",
			file: "or-tool.chunk.json",
			want: &llm.Response{
				ID:      "gen-1761367322-PVxDxpFLbMP0mk86V719",
				Model:   "minimax/minimax-m2:free",
				Object:  "chat.completion.chunk",
				Created: 1761367322,
				Choices: []llm.Choice{
					{
						Index:        0,
						FinishReason: lo.ToPtr("tool_calls"),
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("<think>\n好的，现在我需要仔细review这个文件。让我再次查看文件内容并分析潜在的问题。\n</think>"),
							},
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_function_7280296200_1",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "Read",
										Arguments: "{\"file_path\": \"/axonhub/internal/llm/transformer/openrouter/outbound.go\"}",
									},
								},
							},
						},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     18810,
					CompletionTokens: 68,
					TotalTokens:      18878,
					PromptTokensDetails: &llm.PromptTokensDetails{
						AudioTokens:  0,
						CachedTokens: 0,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						AudioTokens:              0,
						ReasoningTokens:          0,
						AcceptedPredictionTokens: 0,
						RejectedPredictionTokens: 0,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r openrouter.Response

			err := xtest.LoadTestData(t, tt.file, &r)
			require.NoError(t, err)

			got := r.ToOpenAIResponse().ToLLMResponse()
			require.Equal(t, tt.want, got)
		})
	}
}
