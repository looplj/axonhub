package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
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
		{
			name:         "response with cache usage",
			responseFile: "anthropic-cache-usage.response.json",
			expectedFile: "llm-cache-usage.response.json",
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

				// WriteCachedTokens is a hidden field (json:"-"), so we need to set it manually
				// based on the Anthropic response's cache_creation_input_tokens
				if expected.Usage != nil && expected.Usage.PromptTokensDetails != nil && anthropicResp.Usage != nil {
					expected.Usage.PromptTokensDetails.WriteCachedTokens = anthropicResp.Usage.CacheCreationInputTokens
				}

				if !xtest.Equal(expected, *result) {
					t.Fatalf("responses are not equal %s", cmp.Diff(expected, *result))
				}
			}

			if tt.validateResponse != nil {
				tt.validateResponse(t, result)
			}
		})
	}
}
