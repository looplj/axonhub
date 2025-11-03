package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestOutboundTransformer_TransformResponse_WithTestData(t *testing.T) {
	tests := []struct {
		name             string
		responseFile     string
		expectedFile     string
		platformType     PlatformType
		validateResponse func(t *testing.T, resp *llm.Response)
	}{
		{
			name:         "response with stop finish reason",
			responseFile: "anthropic-stop.response.json",
			expectedFile: "llm-stop.response.json",
			platformType: PlatformDirect,
		},
		{
			name:         "response with thinking and tool calls",
			responseFile: "anthropic-think.response.json",
			expectedFile: "llm-think.response.json",
			platformType: PlatformDirect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var anthropicResp Message

			err := xtest.LoadTestData(t, tt.responseFile, &anthropicResp)
			require.NoError(t, err)

			transformer, err := NewOutboundTransformer("", "")
			require.NoError(t, err)

			body, err := json.Marshal(anthropicResp)
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: 200,
				Body:       body,
			}

			result, err := transformer.TransformResponse(t.Context(), httpResp)
			require.NoError(t, err)

			if tt.expectedFile != "" {
				var expected llm.Response

				err = xtest.LoadTestData(t, tt.expectedFile, &expected)
				require.NoError(t, err)

				// Normalize pointer fields for comparison
				normalizeResponseForComparison(t, &expected)
				normalizeResponseForComparison(t, result)

				require.Equal(t, expected, *result)
			}

			if tt.validateResponse != nil {
				tt.validateResponse(t, result)
			}
		})
	}
}

func normalizeResponseForComparison(t *testing.T, resp *llm.Response) {
	t.Helper()

	for i := range resp.Choices {
		choice := &resp.Choices[i]

		if choice.Message != nil {
			if choice.Message.Content.Content != nil {
				trimmed := *choice.Message.Content.Content
				choice.Message.Content.Content = &trimmed
			}

			for j := range choice.Message.Content.MultipleContent {
				part := &choice.Message.Content.MultipleContent[j]
				if part.Text != nil {
					trimmed := *part.Text
					part.Text = &trimmed
				}
			}

			for j := range choice.Message.ToolCalls {
				call := &choice.Message.ToolCalls[j]
				if len(call.Function.Arguments) > 0 {
					var m map[string]any

					err := json.Unmarshal([]byte(call.Function.Arguments), &m)
					require.NoError(t, err)

					normalizedArgs, err := json.Marshal(m)
					require.NoError(t, err)

					call.Function.Arguments = string(normalizedArgs)
				}
			}
		}
	}
}
