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
