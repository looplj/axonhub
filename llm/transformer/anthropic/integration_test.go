package anthropic

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
)

func TestAnthropicTransformers_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name                    string
		anthropicRequestJSON    string
		expectedModel           string
		expectedMaxTokens       int64
		expectedThinkingDisplay string
	}{
		{
			name: "simple text message",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": "Hello, Claude!"
					}
				]
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 1024,
		},
		{
			name: "message with system prompt",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 2048,
				"system": "You are a helpful assistant.",
				"messages": [
					{
						"role": "user",
						"content": "What is the capital of France?"
					}
				],
				"temperature": 0.7
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 2048,
		},
		{
			name: "multimodal message",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{
								"type": "text",
								"text": "What's in this image?"
							},
							{
								"type": "image",
								"source": {
									"type": "base64",
									"media_type": "image/jpeg",
									"data": "/9j/4AAQSkZJRg..."
								}
							}
						]
					}
				]
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 1024,
		},
		{
			name: "thinking with display summarized",
			anthropicRequestJSON: `{
				"model": "claude-sonnet-4-20250514",
				"max_tokens": 8096,
				"thinking": {
					"type": "enabled",
					"budget_tokens": 5000,
					"display": "summarized"
				},
				"messages": [
					{
						"role": "user",
						"content": "Hello"
					}
				]
			}`,
			expectedModel:           "claude-sonnet-4-20250514",
			expectedMaxTokens:       8096,
			expectedThinkingDisplay: "summarized",
		},
		{
			name: "thinking with display omitted",
			anthropicRequestJSON: `{
				"model": "claude-sonnet-4-20250514",
				"max_tokens": 4096,
				"thinking": {
					"type": "enabled",
					"budget_tokens": 10000,
					"display": "omitted"
				},
				"messages": [
					{
						"role": "user",
						"content": "Hello"
					}
				]
			}`,
			expectedModel:           "claude-sonnet-4-20250514",
			expectedMaxTokens:       4096,
			expectedThinkingDisplay: "omitted",
		},
		{
			name: "adaptive thinking with display summarized",
			anthropicRequestJSON: `{
				"model": "claude-sonnet-4-20250514",
				"max_tokens": 4096,
				"thinking": {
					"type": "adaptive",
					"display": "summarized"
				},
				"messages": [
					{
						"role": "user",
						"content": "Hello"
					}
				]
			}`,
			expectedModel:           "claude-sonnet-4-20250514",
			expectedMaxTokens:       4096,
			expectedThinkingDisplay: "summarized",
		},
		{
			name: "disabled thinking ignores display",
			anthropicRequestJSON: `{
				"model": "claude-sonnet-4-20250514",
				"max_tokens": 4096,
				"thinking": {
					"type": "disabled",
					"display": "summarized"
				},
				"messages": [
					{
						"role": "user",
						"content": "Hello"
					}
				]
			}`,
			expectedModel:           "claude-sonnet-4-20250514",
			expectedMaxTokens:       4096,
			expectedThinkingDisplay: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Transform Anthropic request to ChatCompletionRequest
			httpReq := &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(tt.anthropicRequestJSON),
			}

			chatReq, err := inboundTransformer.TransformRequest(t.Context(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			// Verify the transformation
			require.Equal(t, tt.expectedModel, chatReq.Model)
			require.Equal(t, tt.expectedMaxTokens, *chatReq.MaxTokens)
			require.NotEmpty(t, chatReq.Messages)

			// Step 2: Transform ChatCompletionRequest to Anthropic outbound request
			outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
			require.NoError(t, err)
			require.NotNil(t, outboundReq)

			// Verify outbound request
			require.Equal(t, http.MethodPost, outboundReq.Method)
			require.Equal(t, "https://api.anthropic.com/v1/messages", outboundReq.URL)
			require.Equal(t, "application/json", outboundReq.Headers.Get("Content-Type"))
			require.Equal(t, "2023-06-01", outboundReq.Headers.Get("Anthropic-Version"))

			// Verify the outbound request body can be unmarshaled
			var anthropicReq MessageRequest

			err = json.Unmarshal(outboundReq.Body, &anthropicReq)
			require.NoError(t, err)
			require.Equal(t, tt.expectedModel, anthropicReq.Model)
			require.Equal(t, tt.expectedMaxTokens, anthropicReq.MaxTokens)

			// Verify thinking display round-trip
			if tt.expectedThinkingDisplay != "" {
				require.NotNil(t, anthropicReq.Thinking)
				require.Equal(t, tt.expectedThinkingDisplay, anthropicReq.Thinking.Display)
			} else if anthropicReq.Thinking != nil {
				require.Empty(t, anthropicReq.Thinking.Display)
			}

			// Step 3: Simulate Anthropic response and transform back
			anthropicResponse := &Message{
				ID:   "msg_test_123",
				Type: "message",
				Role: "assistant",
				Content: []MessageContentBlock{
					{
						Type: "text",
						Text: lo.ToPtr("This is a test response from Claude."),
					},
				},
				Model:      tt.expectedModel,
				StopReason: func() *string { s := "end_turn"; return &s }(),
				Usage: &Usage{
					InputTokens:  15,
					OutputTokens: 25,
				},
			}

			responseBody, err := json.Marshal(anthropicResponse)
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseBody,
			}

			// Step 4: Transform Anthropic response to ChatCompletionResponse
			chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			// Verify chat response
			require.Equal(t, "msg_test_123", chatResp.ID)
			require.Equal(t, "chat.completion", chatResp.Object)
			require.Equal(t, tt.expectedModel, chatResp.Model)
			require.Equal(t, 1, len(chatResp.Choices))
			require.Equal(t, "assistant", chatResp.Choices[0].Message.Role)
			require.Equal(
				t,
				"This is a test response from Claude.",
				*chatResp.Choices[0].Message.Content.Content,
			)
			require.Equal(t, "stop", *chatResp.Choices[0].FinishReason)

			// Step 5: Transform ChatCompletionResponse back to Anthropic format
			finalHttpResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
			require.NoError(t, err)
			require.NotNil(t, finalHttpResp)

			// Verify final response
			require.Equal(t, http.StatusOK, finalHttpResp.StatusCode)
			require.Equal(t, "application/json", finalHttpResp.Headers.Get("Content-Type"))

			var finalAnthropicResp Message

			err = json.Unmarshal(finalHttpResp.Body, &finalAnthropicResp)
			require.NoError(t, err)
			require.Equal(t, "msg_test_123", finalAnthropicResp.ID)
			require.Equal(t, "message", finalAnthropicResp.Type)
			require.Equal(t, "assistant", finalAnthropicResp.Role)
			require.Equal(t, tt.expectedModel, finalAnthropicResp.Model)
		})
	}
}

func TestAnthropicTransformResponse_CitationRoundTripIntegration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	anthropicResponse := &Message{
		ID:   "msg_citation_roundtrip",
		Type: "message",
		Role: "assistant",
		Content: []MessageContentBlock{
			{
				Type: "text",
				Text: lo.ToPtr("Answer with source"),
				Citations: []TextCitation{
					{
						Type:           "url_citation",
						URL:            "https://example.com/anthropic",
						Title:          "Anthropic Source",
						EncryptedIndex: lo.ToPtr("secret-index"),
						CitedText:      lo.ToPtr("quoted text"),
					},
				},
			},
		},
		Model: "claude-3-7-sonnet-latest",
	}

	responseBody, err := json.Marshal(anthropicResponse)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       responseBody,
	}

	chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, chatResp)
	require.Len(t, chatResp.Choices, 1)
	require.NotNil(t, chatResp.Choices[0].Message)
	require.Equal(t, "assistant", chatResp.Choices[0].Message.Role)
	require.Equal(t, "Answer with source", lo.FromPtr(chatResp.Choices[0].Message.Content.Content))
	require.Len(t, chatResp.Choices[0].Message.Annotations, 1)

	annotation := chatResp.Choices[0].Message.Annotations[0]
	require.Equal(t, "url_citation", annotation.Type)
	require.NotNil(t, annotation.URLCitation)
	require.Equal(t, "https://example.com/anthropic", annotation.URLCitation.URL)
	require.Equal(t, "Anthropic Source", annotation.URLCitation.Title)
	require.Nil(t, annotation.StartIndex)
	require.Nil(t, annotation.EndIndex)

	finalHTTPResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
	require.NoError(t, err)
	require.NotNil(t, finalHTTPResp)

	var finalAnthropicResp Message

	err = json.Unmarshal(finalHTTPResp.Body, &finalAnthropicResp)
	require.NoError(t, err)
	require.Equal(t, "msg_citation_roundtrip", finalAnthropicResp.ID)
	require.Equal(t, "message", finalAnthropicResp.Type)
	require.Equal(t, "assistant", finalAnthropicResp.Role)
	require.Equal(t, "claude-3-7-sonnet-latest", finalAnthropicResp.Model)
	require.Len(t, finalAnthropicResp.Content, 1)
	require.Equal(t, "text", finalAnthropicResp.Content[0].Type)
	require.Equal(t, "Answer with source", lo.FromPtr(finalAnthropicResp.Content[0].Text))
	require.Len(t, finalAnthropicResp.Content[0].Citations, 1)

	citation := finalAnthropicResp.Content[0].Citations[0]
	require.Equal(t, "url_citation", citation.Type)
	require.Equal(t, "https://example.com/anthropic", citation.URL)
	require.Equal(t, "Anthropic Source", citation.Title)
	require.Nil(t, citation.EncryptedIndex)
	require.Nil(t, citation.CitedText)
}


func TestAnthropicTransformResponse_WebSearchBlocks_RoundTripIntegration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	anthropicResponse := &Message{
		ID:   "msg_a930390d3a",
		Type: "message",
		Role: "assistant",
		Content: []MessageContentBlock{
			{
				Type: "text",
				Text: lo.ToPtr("I'll search for when Claude Shannon was born."),
			},
			{
				Type:  "server_tool_use",
				ID:    "srvtoolu_01WYG3ziw53XMcoyKL4XcZmE",
				Name:  lo.ToPtr("web_search"),
				Input: json.RawMessage(`{"query":"claude shannon birth date"}`),
			},
			{
				Type:      "web_search_tool_result",
				ToolUseID: lo.ToPtr("srvtoolu_01WYG3ziw53XMcoyKL4XcZmE"),
				Content: &MessageContent{MultipleContent: []MessageContentBlock{{
					Type:             "web_search_result",
					URL:              "https://en.wikipedia.org/wiki/Claude_Shannon",
					Title:            "Claude Shannon - Wikipedia",
					EncryptedContent: lo.ToPtr("EqgfCioIARgBIiQ3YTAwMjY1Mi1mZjM5LTQ1NGUtODgxNC1kNjNjNTk1ZWI3Y..."),
					PageAge:          lo.ToPtr("April 30, 2025"),
				}}},
			},
			{
				Type: "text",
				Text: lo.ToPtr("Based on the search results, "),
			},
			{
				Type: "text",
				Text: lo.ToPtr("Claude Shannon was born on April 30, 1916, in Petoskey, Michigan"),
				Citations: []TextCitation{{
					Type:           "web_search_result_location",
					URL:            "https://en.wikipedia.org/wiki/Claude_Shannon",
					Title:          "Claude Shannon - Wikipedia",
					EncryptedIndex: lo.ToPtr("Eo8BCioIAhgBIiQyYjQ0OWJmZi1lNm.."),
					CitedText:      lo.ToPtr("Claude Elwood Shannon (April 30, 1916 – February 24, 2001) was an American mathematician, electrical engineer, computer scientist, cryptographer and i..."),
				}},
			},
		},
		Model: "claude-3-7-sonnet-latest",
		Usage: &Usage{InputTokens: 6039, OutputTokens: 931},
	}

	responseBody, err := json.Marshal(anthropicResponse)
	require.NoError(t, err)

	httpResp := &httpclient.Response{StatusCode: http.StatusOK, Body: responseBody}
	chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, chatResp)
	require.Len(t, chatResp.Choices, 1)
	require.NotNil(t, chatResp.Choices[0].Message)
	require.Len(t, chatResp.Choices[0].Message.Annotations, 1)

	finalHTTPResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
	require.NoError(t, err)
	require.NotNil(t, finalHTTPResp)

	root := gjson.ParseBytes(finalHTTPResp.Body)
	content := root.Get("content")
	require.True(t, content.Exists())
	require.Len(t, content.Array(), 5)

	require.Equal(t, "text", content.Array()[0].Get("type").String())
	require.Equal(t, "I'll search for when Claude Shannon was born.", content.Array()[0].Get("text").String())

	require.Equal(t, "server_tool_use", content.Array()[1].Get("type").String())
	require.Equal(t, "srvtoolu_01WYG3ziw53XMcoyKL4XcZmE", content.Array()[1].Get("id").String())
	require.Equal(t, "web_search", content.Array()[1].Get("name").String())
	require.Equal(t, "claude shannon birth date", content.Array()[1].Get("input.query").String())

	require.Equal(t, "web_search_tool_result", content.Array()[2].Get("type").String())
	require.Equal(t, "srvtoolu_01WYG3ziw53XMcoyKL4XcZmE", content.Array()[2].Get("tool_use_id").String())
	require.Len(t, content.Array()[2].Get("content").Array(), 1)
	require.Equal(t, "web_search_result", content.Array()[2].Get("content.0.type").String())
	require.Equal(t, "https://en.wikipedia.org/wiki/Claude_Shannon", content.Array()[2].Get("content.0.url").String())
	require.Equal(t, "Claude Shannon - Wikipedia", content.Array()[2].Get("content.0.title").String())
	require.Equal(t, "EqgfCioIARgBIiQ3YTAwMjY1Mi1mZjM5LTQ1NGUtODgxNC1kNjNjNTk1ZWI3Y...", content.Array()[2].Get("content.0.encrypted_content").String())
	require.Equal(t, "April 30, 2025", content.Array()[2].Get("content.0.page_age").String())

	require.Equal(t, "text", content.Array()[3].Get("type").String())
	require.Equal(t, "Based on the search results, ", content.Array()[3].Get("text").String())

	require.Equal(t, "text", content.Array()[4].Get("type").String())
	require.Equal(t, "Claude Shannon was born on April 30, 1916, in Petoskey, Michigan", content.Array()[4].Get("text").String())
	require.Len(t, content.Array()[4].Get("citations").Array(), 1)
	require.Equal(t, "web_search_result_location", content.Array()[4].Get("citations.0.type").String())
	require.Equal(t, "https://en.wikipedia.org/wiki/Claude_Shannon", content.Array()[4].Get("citations.0.url").String())
	require.Equal(t, "Claude Shannon - Wikipedia", content.Array()[4].Get("citations.0.title").String())
	require.Equal(t, "Eo8BCioIAhgBIiQyYjQ0OWJmZi1lNm..", content.Array()[4].Get("citations.0.encrypted_index").String())
	require.Equal(t, "Claude Elwood Shannon (April 30, 1916 – February 24, 2001) was an American mathematician, electrical engineer, computer scientist, cryptographer and i...", content.Array()[4].Get("citations.0.cited_text").String())
}

func TestTransformRequest_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name        string
		requestFile string
	}{
		{
			name:        "claude code",
			requestFile: `anthropic-claude-code.request.json`,
		},
		{
			name:        "claude code2",
			requestFile: `anthropic-claude-code2.request.json`,
		},
		{
			name:        "claude thinking",
			requestFile: `anthropic-thinking.request.json`,
		},
		{
			name:        "tool result with reasoning",
			requestFile: `anthropic-tool-result-mixed.request.json`,
		},
		{
			name:        "1 item system array request",
			requestFile: `anthropic-system-1.request.json`,
		},
		{
			name:        "parallel multiple tool request",
			requestFile: `anthropic-parallel_multiple_tool.request.json`,
		},
		{
			name:        "parallel2 multiple tool request",
			requestFile: `anthropic-parallel2_multiple_tool.request.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wantReq MessageRequest

			err := xtest.LoadTestData(t, tt.requestFile, &wantReq)
			require.NoError(t, err)

			var buf bytes.Buffer

			decoder := json.NewEncoder(&buf)
			decoder.SetEscapeHTML(false)

			if err := decoder.Encode(wantReq); err != nil {
				t.Fatalf("failed to marshal tool result: %v", err)
			}

			chatReq, err := inboundTransformer.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: buf.Bytes(),
			})
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
			require.NoError(t, err)

			var gotReq MessageRequest

			err = json.Unmarshal(outboundReq.Body, &gotReq)
			require.NoError(t, err)

			// 忽略 cache_control 差异：ensureCacheControl 会在 outbound 路径中自动注入断点，
			// 可能导致 CacheControl 字段和 Content→MultipleContent 结构变化。
			// 这些行为的正确性已在 ensure_cache_control_test.go 中覆盖。
			if !xtest.Equal(wantReq, gotReq, ignoreCacheControlWithNormalize...) {
				t.Errorf("wantReq != gotReq\n%s", cmp.Diff(wantReq, gotReq, ignoreCacheControlWithNormalize...))
			}
		})
	}

	// 单独测试 cache_control 超限的 fixture：该文件包含 6 个 cache_control 断点。
	// strict mode 下会重建为结构锚点 + 受预算约束的消息锚点（此场景为 3 个）。
	t.Run("cache control exceeds limit", func(t *testing.T) {
		var wantReq MessageRequest

		err := xtest.LoadTestData(t, "anthropic-cache-control-inbound.request.json", &wantReq)
		require.NoError(t, err)

		var buf bytes.Buffer

		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		require.NoError(t, encoder.Encode(wantReq))

		chatReq, err := inboundTransformer.TransformRequest(t.Context(), &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: buf.Bytes(),
		})
		require.NoError(t, err)

		outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)

		var gotReq MessageRequest

		err = json.Unmarshal(outboundReq.Body, &gotReq)
		require.NoError(t, err)
		require.Equal(t, 3, countCacheControls(&gotReq))
	})
}

func TestAnthropicTransformers_StreamingIntegration(t *testing.T) {
	outboundTransformer, _ := NewOutboundTransformer("https://api.claude.com", "xxx")

	// Simulate streaming chunks from Anthropic
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{
				"type": "message_start",
				"message": {
					"id": "msg_stream_123",
					"type": "message",
					"role": "assistant",
					"content": [],
					"model": "claude-3-sonnet-20240229",
					"stop_reason": null,
					"stop_sequence": null,
					"usage": {"input_tokens": 10, "output_tokens": 0}
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_start",
				"index": 0,
				"content_block": {
					"type": "text",
					"text": ""
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": "Hello"
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": ", this is"
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": " a streaming response!"
				}
			}`),
		},
		{
			Data: []byte(`{
				"type": "content_block_stop",
				"index": 0
			}`),
		},
		{
			Data: []byte(`{
				"type": "message_delta",
				"delta": {
					"stop_reason": "end_turn",
					"stop_sequence": null
				},
				"usage": {"input_tokens": 10, "output_tokens": 25}
			}`),
		},
		{
			Data: []byte(`{
				"type": "message_stop"
			}`),
		},
	}

	// Aggregate the streaming chunks
	chatRespBytes, _, err := outboundTransformer.AggregateStreamChunks(t.Context(), nil, chunks)
	require.NoError(t, err)
	require.NotNil(t, chatRespBytes)

	// Parse the response
	var chatResp Message

	err = json.Unmarshal(chatRespBytes, &chatResp)
	require.NoError(t, err)

	// Verify the aggregated response
	require.Equal(t, "msg_stream_123", chatResp.ID)
	require.Equal(t, "message", chatResp.Type)
	require.Equal(t, 1, len(chatResp.Content))
	require.Equal(t, "assistant", chatResp.Role)
	require.Equal(
		t,
		"Hello, this is a streaming response!",
		*chatResp.Content[0].Text,
	)
	require.NotNil(t, chatResp.StopReason)
	require.Equal(t, "end_turn", *chatResp.StopReason)

	// Verify usage
	require.NotNil(t, chatResp.Usage)
	require.Equal(t, int64(10), chatResp.Usage.InputTokens)
	require.Equal(t, int64(25), chatResp.Usage.OutputTokens)
}

func TestTransformResponse_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name         string
		requestFile  string
		expectedFile string
	}{
		{
			name:         "anthropic-tool.response.json",
			requestFile:  `anthropic-tool.response.json`,
			expectedFile: `anthropic-tool.response.json`,
		},
		{
			name:         "anthropic-think.response.json",
			requestFile:  `anthropic-think.response.json`,
			expectedFile: `anthropic-think.response.json`,
		},
		{
			name:         "anthropic-tool2.response.json",
			requestFile:  `anthropic-tool2.response.json`,
			expectedFile: `anthropic-tool2.response.json`,
		},
		{
			name:         "anthropic-stop.response.json",
			requestFile:  `anthropic-stop.response.json`,
			expectedFile: `anthropic-stop.response.json`,
		},
		{
			name:         "anthropic-cache-usage.response.json",
			requestFile:  `anthropic-cache-usage.response.json`,
			expectedFile: `anthropic-cache-usage.response.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputMessage Message

			err := xtest.LoadTestData(t, tt.requestFile, &inputMessage)
			require.NoError(t, err)

			var expectedMessage Message

			err = xtest.LoadTestData(t, tt.expectedFile, &expectedMessage)
			require.NoError(t, err)

			var buf bytes.Buffer

			encoder := json.NewEncoder(&buf)
			encoder.SetEscapeHTML(false)

			if err := encoder.Encode(inputMessage); err != nil {
				t.Fatalf("failed to marshal tool result: %v", err)
			}

			chatResp, err := outboundTransformer.TransformResponse(t.Context(), &httpclient.Response{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: buf.Bytes(),
			})
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			inboundResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
			require.NoError(t, err)

			var gotMessage Message

			err = json.Unmarshal(inboundResp.Body, &gotMessage)
			require.NoError(t, err)

			if !xtest.Equal(expectedMessage, gotMessage, cmpopts.IgnoreFields(MessageContentBlock{}, "Signature")) {
				t.Errorf("wantMessage != gotMessage\n%s", cmp.Diff(expectedMessage, gotMessage))
			}
		})
	}
}
