package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestOutboundTransformer_TransformResponse_Integration(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	tests := []struct {
		name             string
		responseFile     string // OpenAI Responses API format (input)
		expectedFile     string // LLM format (expected output)
		validateResponse func(t *testing.T, result *llm.Response, expected *llm.Response)
	}{
		{
			name:         "simple text response transformation",
			responseFile: "simple.response.json",
			expectedFile: "llm-simple.response.json",
			validateResponse: func(t *testing.T, result *llm.Response, expected *llm.Response) {
				t.Helper()

				require.Equal(t, expected.Object, result.Object)
				require.Equal(t, expected.ID, result.ID)
				require.Equal(t, expected.Model, result.Model)
				require.Len(t, result.Choices, len(expected.Choices))

				if len(expected.Choices) > 0 && expected.Choices[0].Message != nil {
					require.NotNil(t, result.Choices[0].Message)
					require.Equal(t, expected.Choices[0].Message.Role, result.Choices[0].Message.Role)

					// Compare content
					if expected.Choices[0].Message.Content.Content != nil {
						require.NotNil(t, result.Choices[0].Message.Content.Content)
						require.Equal(t, *expected.Choices[0].Message.Content.Content,
							*result.Choices[0].Message.Content.Content)
					}
				}

				// Verify usage
				if expected.Usage != nil {
					require.NotNil(t, result.Usage)
					require.Equal(t, expected.Usage.PromptTokens, result.Usage.PromptTokens)
					require.Equal(t, expected.Usage.CompletionTokens, result.Usage.CompletionTokens)
					require.Equal(t, expected.Usage.TotalTokens, result.Usage.TotalTokens)
				}
			},
		},
		{
			name:         "tool call response transformation",
			responseFile: "tool.response.json",
			expectedFile: "llm-tool.response.json",
			validateResponse: func(t *testing.T, result *llm.Response, expected *llm.Response) {
				t.Helper()

				require.Equal(t, expected.Object, result.Object)
				require.Equal(t, expected.Model, result.Model)
				require.Len(t, result.Choices, len(expected.Choices))

				if len(expected.Choices) > 0 && expected.Choices[0].Message != nil {
					require.NotNil(t, result.Choices[0].Message)

					// Verify tool calls
					require.Len(t, result.Choices[0].Message.ToolCalls, len(expected.Choices[0].Message.ToolCalls))

					for i, expectedTC := range expected.Choices[0].Message.ToolCalls {
						actualTC := result.Choices[0].Message.ToolCalls[i]
						require.Equal(t, expectedTC.ID, actualTC.ID)
						require.Equal(t, expectedTC.Type, actualTC.Type)
						require.Equal(t, expectedTC.Function.Name, actualTC.Function.Name)
						require.Equal(t, expectedTC.Function.Arguments, actualTC.Function.Arguments)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the OpenAI Responses API response
			responseData, err := loadTestDataRaw(t, tt.responseFile)
			if err != nil {
				t.Skipf("Test data file %s not found, skipping test", tt.responseFile)

				return
			}

			// Create HTTP response
			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseData,
			}

			// Transform the response
			result, err := trans.TransformResponse(t.Context(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Load expected LLM response
			var expected llm.Response

			err = xtest.LoadTestData(t, tt.expectedFile, &expected)
			require.NoError(t, err)

			// Run validation
			tt.validateResponse(t, result, &expected)
		})
	}
}

func TestOutboundTransformer_TransformRequest_Integration(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	tests := []struct {
		name         string
		requestFile  string // LLM format (input)
		expectedFile string // OpenAI Responses API format (expected output - for structure reference)
		validate     func(t *testing.T, result *httpclient.Request, llmReq *llm.Request)
	}{
		{
			name:         "simple text request transformation",
			requestFile:  "llm-simple.request.json",
			expectedFile: "simple.request.json",
			validate: func(t *testing.T, result *httpclient.Request, llmReq *llm.Request) {
				t.Helper()

				// Verify HTTP request properties
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.openai.com/responses", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.NotNil(t, result.Auth)
				require.Equal(t, "bearer", result.Auth.Type)

				// Parse the transformed request body
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)

				// Verify model
				require.Equal(t, llmReq.Model, req.Model)

				// Verify input is properly structured
				require.NotNil(t, req.Input)
			},
		},
		{
			name:         "tool request transformation",
			requestFile:  "llm-tool.request.json",
			expectedFile: "tool.request.json",
			validate: func(t *testing.T, result *httpclient.Request, llmReq *llm.Request) {
				t.Helper()

				// Parse the transformed request body
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)

				// Verify model
				require.Equal(t, llmReq.Model, req.Model)

				// Verify tools are properly transformed
				if len(llmReq.Tools) > 0 {
					require.NotNil(t, req.Tools)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the LLM request
			var llmReq llm.Request

			err := xtest.LoadTestData(t, tt.requestFile, &llmReq)
			if err != nil {
				t.Skipf("Test data file %s not found, skipping test", tt.requestFile)

				return
			}

			// Transform the request
			result, err := trans.TransformRequest(t.Context(), &llmReq)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result, &llmReq)
		})
	}
}
