package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestInboundTransformer_TransformRequest_WithTestData(t *testing.T) {
	tests := []struct {
		name         string
		requestFile  string
		expectedFile string
		validate     func(t *testing.T, result *llm.Request, httpReq *httpclient.Request)
	}{
		{
			name:         "simple text request transformation",
			requestFile:  "simple.request.json",
			expectedFile: "llm-simple.request.json",
			validate: func(t *testing.T, result *llm.Request, httpReq *httpclient.Request) {
				t.Helper()

				// Verify basic request properties
				require.Equal(t, "deepseek-chat", result.Model)
				require.Equal(t, llm.APIFormatOpenAIResponse, result.RawAPIFormat)

				// Verify messages
				require.Len(t, result.Messages, 7)
				require.Equal(t, "user", result.Messages[0].Role)

				// For single input_text, content should be a simple string (optimized path)
				require.NotNil(t, result.Messages[0].Content.Content)
				require.Equal(t, "My name is Alice.", *result.Messages[0].Content.Content)
				require.Nil(t, result.Messages[0].Content.MultipleContent)
			},
		},
		{
			name:         "tool request transformation",
			requestFile:  "tool.request.json",
			expectedFile: "llm-tool.request.json",
			validate: func(t *testing.T, result *llm.Request, httpReq *httpclient.Request) {
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the test request data as raw JSON
			var reqData json.RawMessage

			err := xtest.LoadTestData(t, tt.requestFile, &reqData)
			require.NoError(t, err)

			// Create HTTP request with the loaded data
			httpReq := &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: reqData,
			}

			// Create transformer
			transformer := NewInboundTransformer()

			// Transform the request
			result, err := transformer.TransformRequest(t.Context(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result, httpReq)

			var expected llm.Request

			err = xtest.LoadTestData(t, tt.expectedFile, &expected)
			require.NoError(t, err)

			expected.RawAPIFormat = llm.APIFormatOpenAIResponse
			if !xtest.Equal(expected, *result) {
				t.Errorf("diff: %v", cmp.Diff(expected, *result))
			}
		})
	}
}

func TestInboundTransformer_TransformResponse_WithTestData(t *testing.T) {
	tests := []struct {
		name         string
		responseFile string // LLM response format (input)
		expectedFile string // OpenAI Responses API format (expected output)
		validate     func(t *testing.T, result *httpclient.Response, resp *Response)
	}{
		{
			name:         "simple text response transformation",
			responseFile: "llm-simple.response.json",
			expectedFile: "simple.response.json",
			validate: func(t *testing.T, result *httpclient.Response, resp *Response) {
				t.Helper()

				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))

				// Verify response properties
				require.Equal(t, "response", resp.Object)
				require.Equal(t, "gpt-4o", resp.Model)
				require.NotNil(t, resp.Status)
				require.Equal(t, "completed", *resp.Status)

				// Verify output
				require.Len(t, resp.Output, 1)
				output := resp.Output[0]
				require.Equal(t, "message", output.Type)
				require.Equal(t, "assistant", output.Role)
				require.Len(t, output.GetContentItems(), 1)
				require.Equal(t, "output_text", output.GetContentItems()[0].Type)
			},
		},
		{
			name:         "tool call response transformation",
			responseFile: "llm-tool.response.json",
			expectedFile: "tool.response.json",
			validate: func(t *testing.T, result *httpclient.Response, resp *Response) {
				t.Helper()

				require.Equal(t, http.StatusOK, result.StatusCode)

				// Verify response properties
				require.Equal(t, "response", resp.Object)
				require.NotNil(t, resp.Status)
				require.Equal(t, "completed", *resp.Status)

				// Verify tool call outputs
				require.Len(t, resp.Output, 2)

				// First tool call
				output0 := resp.Output[0]
				require.Equal(t, "function_call", output0.Type)
				require.Equal(t, "call_eda8722c71944fe394a8893c0de8146a", output0.ID)

				// Second tool call
				output1 := resp.Output[1]
				require.Equal(t, "function_call", output1.Type)
				require.Equal(t, "call_bd313747960f44af8bef50dc27f0f07e", output1.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the LLM response data
			var llmResp llm.Response

			err := xtest.LoadTestData(t, tt.responseFile, &llmResp)
			require.NoError(t, err)

			// Create transformer
			transformer := NewInboundTransformer()

			// Transform the response
			result, err := transformer.TransformResponse(t.Context(), &llmResp)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Parse the result body
			var resp Response

			err = json.Unmarshal(result.Body, &resp)
			require.NoError(t, err)

			// Run validation
			tt.validate(t, result, &resp)

			// Load expected response and compare
			var expected Response

			err = xtest.LoadTestData(t, tt.expectedFile, &expected)
			require.NoError(t, err)

			// Compare with ignoring dynamic fields (IDs generated at runtime)
			// Since Output is []Item, we need to ignore the ID field in Item structs
			opts := cmp.FilterPath(func(p cmp.Path) bool {
				// Ignore "ID" field in Item structs within Output array
				if len(p) >= 2 {
					if sf, ok := p[len(p)-1].(cmp.StructField); ok {
						if sf.Name() == "ID" {
							return true
						}
					}
				}

				return false
			}, cmp.Ignore())
			if diff := cmp.Diff(expected, resp, opts); diff != "" {
				t.Errorf("response mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}
