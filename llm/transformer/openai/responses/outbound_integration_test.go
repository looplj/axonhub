package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
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
			var responseData json.RawMessage

			err := xtest.LoadTestData(t, tt.responseFile, &responseData)
			if err != nil {
				t.Errorf("Test data file %s not found, skipping test", tt.responseFile)
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

				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)

				// Verify compaction item exists in input
				require.Len(t, req.Input.Items, 8)
				compactionItem := req.Input.Items[6]
				require.Equal(t, "compaction", compactionItem.Type)
				require.NotNil(t, compactionItem.EncryptedContent)
				require.Equal(t, "gAAAAABpxygtxqpBeKM2Wvlv2Owja3cpZk2rbpgr8iXCl9Zhl7JAJCVy7nIP===", *compactionItem.EncryptedContent)
			},
		},
		{
			name:         "tool request transformation",
			requestFile:  "llm-tool.request.json",
			expectedFile: "tool.request.json",
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
			actualResult, err := trans.TransformRequest(t.Context(), &llmReq)
			require.NoError(t, err)
			require.NotNil(t, actualResult)

			// Run validation
			if tt.validate != nil {
				tt.validate(t, actualResult, &llmReq)
			}

			var expectedRequest Request

			err = xtest.LoadTestData(t, tt.expectedFile, &expectedRequest)
			require.NoError(t, err)

			actualRequest, err := xjson.To[Request](actualResult.Body)
			require.NoError(t, err)

			if !xtest.Equal(expectedRequest, actualRequest) {
				t.Errorf("diff: %v", cmp.Diff(expectedRequest, actualRequest))
			}
		})
	}
}

func TestCompactTransformer_TransformResponse_Integration(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	inbound := NewCompactInboundTransformer()

	var responseData json.RawMessage

	err = xtest.LoadTestData(t, "compact.response.json", &responseData)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       responseData,
		Request: &httpclient.Request{
			RequestType: string(llm.RequestTypeCompact),
		},
	}

	llmResp, err := outbound.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.NotNil(t, llmResp.Compact)
	require.GreaterOrEqual(t, len(llmResp.Compact.Output), 3)
	require.Equal(t, "msg_03f11a6fcfdf35990169c6cbf3a1448191b76d79883bea687a", llmResp.Compact.Output[0].ID)
	require.Equal(t, "developer", llmResp.Compact.Output[0].Role)
	require.Equal(t, "msg_03f11a6fcfdf35990169c6cbf3a150819195277d047c179c05", llmResp.Compact.Output[1].ID)
	require.Equal(t, "user", llmResp.Compact.Output[1].Role)
	require.Equal(t, "msg_03f11a6fcfdf35990169c6cbf3a1588191903ea3ef7d1f82f2", llmResp.Compact.Output[2].ID)
	require.Equal(t, "developer", llmResp.Compact.Output[2].Role)
	require.Len(t, llmResp.Compact.Output, 16)
	lastMsg := llmResp.Compact.Output[len(llmResp.Compact.Output)-1]
	require.Equal(t, "assistant", lastMsg.Role)
	require.Len(t, lastMsg.Content.MultipleContent, 1)
	require.Equal(t, "compaction_summary", lastMsg.Content.MultipleContent[0].Type)
	require.Equal(t, "cmp_03f11a6fcfdf35990169c6cbf468dc8191a9f4bb741308f6b5", lastMsg.Content.MultipleContent[0].ID)

	roundTripResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual CompactAPIResponse

	err = json.Unmarshal(roundTripResp.Body, &actual)
	require.NoError(t, err)

	var expected CompactAPIResponse

	err = xtest.LoadTestData(t, "compact.response.json", &expected)
	require.NoError(t, err)

	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.CreatedAt, actual.CreatedAt)
	require.Equal(t, expected.Object, actual.Object)
	require.Equal(t, expected.Usage, actual.Usage)

	opts := []cmp.Option{
		cmpopts.IgnoreFields(Item{}, "Annotations"),
		cmpopts.EquateEmpty(),
	}
	if diff := cmp.Diff(expected.Output, actual.Output, opts...); diff != "" {
		t.Errorf("diff: %v", diff)
	}
}

func TestResponsesTransformer_TransformResponse_Integration(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	inbound := NewInboundTransformer()

	var responseData json.RawMessage

	err = xtest.LoadTestData(t, "stop.response.json", &responseData)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       responseData,
	}

	llmResp, err := outbound.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].Message)
	require.Equal(t, "assistant", llmResp.Choices[0].Message.Role)
	require.Equal(t, "msg_68daaab83ca881979d9202218c9f957a001f79b13b9c9cbb", llmResp.Choices[0].Message.ID)

	roundTripResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual Response

	err = json.Unmarshal(roundTripResp.Body, &actual)
	require.NoError(t, err)

	var expected Response

	err = xtest.LoadTestData(t, "stop.response.json", &expected)
	require.NoError(t, err)

	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.CreatedAt, actual.CreatedAt)
	require.Equal(t, expected.Object, actual.Object)
	require.Equal(t, expected.Status, actual.Status)
	require.Equal(t, expected.Model, actual.Model)
	require.Equal(t, expected.Usage, actual.Usage)

	opts := []cmp.Option{
		cmpopts.IgnoreFields(Item{}, "Annotations"),
		cmpopts.EquateEmpty(),
	}
	if diff := cmp.Diff(expected.Output, actual.Output, opts...); diff != "" {
		t.Errorf("diff: %v", diff)
	}
}
