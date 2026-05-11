package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

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

func TestOutboundTransformer_TransformRequest_WithWebSearchTool(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	llmReq := &llm.Request{
		Model: "gpt-4o-search-preview",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("what happened today in ai")}},
		},
		Tools: []llm.Tool{{
			Type: llm.ToolTypeWebSearch,
			WebSearch: &llm.WebSearch{
				AllowedDomains: []string{"openai.com", "example.com"},
				UserLocation: llm.WebSearchToolUserLocation{
					Type:     "approximate",
					City:     "San Francisco",
					Country:  "US",
					Region:   "California",
					Timezone: "America/Los_Angeles",
				},
			},
		}},
	}

	hreq, err := trans.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var actual Request
	err = json.Unmarshal(hreq.Body, &actual)
	require.NoError(t, err)
	require.Len(t, actual.Tools, 1)
	require.Equal(t, Tool{
		Type: "web_search",
		Filters: &WebSearchFilters{
			AllowedDomains: []string{"openai.com", "example.com"},
		},
		UserLocation: &WebSearchUserLocation{
			Type:     "approximate",
			City:     "San Francisco",
			Country:  "US",
			Region:   "California",
			Timezone: "America/Los_Angeles",
		},
	}, actual.Tools[0])
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

func TestResponsesTransformer_CitationAnnotations_RoundTripIntegration(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	inbound := NewInboundTransformer()

	responsePayload := Response{
		Object:    "response",
		ID:        "resp_annotations_round_trip",
		CreatedAt: 1759161016,
		Model:     "gpt-4o",
		Status:    lo.ToPtr("completed"),
		Output: []Item{{
			ID:     "msg_annotations_round_trip",
			Type:   "message",
			Status: lo.ToPtr("completed"),
			Role:   "assistant",
			Content: &Input{Items: []Item{
				{Type: "output_text", Text: lo.ToPtr("Alpha ")},
				{
					Type: "output_text",
					Text: lo.ToPtr("Beta"),
					Annotations: []Annotation{{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(0)),
						EndIndex:   lo.ToPtr(int64(4)),
						URLCitation: &URLCitation{
							URL:   "https://example.com/beta",
							Title: "Beta Source",
						},
					}},
				},
			}},
		}},
		Usage: &Usage{InputTokens: 16, OutputTokens: 8, TotalTokens: 24},
	}

	responseData, err := json.Marshal(responsePayload)
	require.NoError(t, err)

	httpResp := &httpclient.Response{StatusCode: http.StatusOK, Body: responseData}
	llmResp, err := outbound.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].Message)
	require.Len(t, llmResp.Choices[0].Message.Annotations, 1)

	annotation := llmResp.Choices[0].Message.Annotations[0]
	require.Equal(t, "url_citation", annotation.Type)
	require.NotNil(t, annotation.URLCitation)
	require.Equal(t, "https://example.com/beta", annotation.URLCitation.URL)
	require.Equal(t, "Beta Source", annotation.URLCitation.Title)
	require.NotNil(t, annotation.StartIndex)
	require.NotNil(t, annotation.EndIndex)
	require.EqualValues(t, 6, *annotation.StartIndex)
	require.EqualValues(t, 10, *annotation.EndIndex)

	roundTripResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var roundTrip Response
	err = json.Unmarshal(roundTripResp.Body, &roundTrip)
	require.NoError(t, err)
	require.Len(t, roundTrip.Output, 1)

	contentItems := roundTrip.Output[0].GetContentItems()
	require.Len(t, contentItems, 1)
	require.Equal(t, "output_text", contentItems[0].Type)
	require.Equal(t, "Alpha Beta", contentItems[0].Text)
	require.Len(t, contentItems[0].Annotations, 1)
	require.Equal(t, "url_citation", contentItems[0].Annotations[0].Type)
	require.NotNil(t, contentItems[0].Annotations[0].StartIndex)
	require.NotNil(t, contentItems[0].Annotations[0].EndIndex)
	require.EqualValues(t, 6, *contentItems[0].Annotations[0].StartIndex)
	require.EqualValues(t, 10, *contentItems[0].Annotations[0].EndIndex)
	require.NotNil(t, contentItems[0].Annotations[0].URLCitation)
	require.Equal(t, "https://example.com/beta", contentItems[0].Annotations[0].URLCitation.URL)
	require.Equal(t, "Beta Source", contentItems[0].Annotations[0].URLCitation.Title)
}

func TestResponsesTransformer_WebSearchCallItem_RoundTripIntegration(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	inbound := NewInboundTransformer()

	responseData := []byte(`{
		"object":"response",
		"id":"resp_web_search_call_round_trip",
		"created_at":1759161016,
		"model":"gpt-4o-search-preview",
		"status":"completed",
		"output":[
			{
				"type":"web_search_call",
				"id":"ws_67c9fa0502748190b7dd390736892e100be649c1a5ff9609",
				"status":"completed",
				"action":{
					"type":"search",
					"query":"latest news about AI",
					"queries":["latest news about AI","AI headlines today"],
					"sources":[
						{"type":"url","url":"https://example.com/news"},
						{"type":"url","url":"https://example.com/analysis","title":"Analysis"}
					]
				}
			},
			{
				"id":"msg_67c9fa077e288190af08fdffda2e34f20be649c1a5ff9609",
				"type":"message",
				"status":"completed",
				"role":"assistant",
				"content":[{
					"type":"output_text",
					"text":"On March 6, 2025, several news...",
					"annotations":[{
						"type":"url_citation",
						"start_index":0,
						"end_index":12,
						"url":"https://example.com/news",
						"title":"Title..."
					}]
				}]
			}
		],
		"usage":{"input_tokens":16,"output_tokens":8,"total_tokens":24}
	}`)

	httpResp := &httpclient.Response{StatusCode: http.StatusOK, Body: responseData}
	llmResp, err := outbound.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].Message)
	require.Equal(t, "On March 6, 2025, several news...", lo.FromPtr(llmResp.Choices[0].Message.Content.Content))

	roundTripResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	root := gjson.ParseBytes(roundTripResp.Body)
	output := root.Get("output")
	require.True(t, output.Exists())
	require.Len(t, output.Array(), 2)

	first := output.Array()[0]
	require.Equal(t, "web_search_call", first.Get("type").String())
	require.Equal(t, "ws_67c9fa0502748190b7dd390736892e100be649c1a5ff9609", first.Get("id").String())
	require.Equal(t, "search", first.Get("action.type").String())
	require.Equal(t, "latest news about AI", first.Get("action.query").String())
	require.Equal(t, []string{"latest news about AI", "AI headlines today"}, []string{
		first.Get("action.queries.0").String(),
		first.Get("action.queries.1").String(),
	})
	require.Len(t, first.Get("action.sources").Array(), 2)
	require.Equal(t, "url", first.Get("action.sources.0.type").String())
	require.Equal(t, "https://example.com/news", first.Get("action.sources.0.url").String())
	require.Equal(t, "url", first.Get("action.sources.1.type").String())
	require.Equal(t, "https://example.com/analysis", first.Get("action.sources.1.url").String())
	require.Equal(t, "Analysis", first.Get("action.sources.1.title").String())
}
