package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCacheControl_ToLLMCacheControl(t *testing.T) {
	t.Run("nil cache control", func(t *testing.T) {
		var cc *CacheControl

		result := convertToLLMCacheControl(cc)
		require.Nil(t, result)
	})

	t.Run("cache control with type only", func(t *testing.T) {
		cc := &CacheControl{
			Type: "ephemeral",
		}
		result := convertToLLMCacheControl(cc)
		require.NotNil(t, result)
		require.Equal(t, "ephemeral", result.Type)
		require.Equal(t, "", result.TTL)
	})

	t.Run("cache control with type and ttl", func(t *testing.T) {
		cc := &CacheControl{
			Type: "ephemeral",
			TTL:  "5m",
		}
		result := convertToLLMCacheControl(cc)
		require.NotNil(t, result)
		require.Equal(t, "ephemeral", result.Type)
		require.Equal(t, "5m", result.TTL)
	})
}

func TestConvertCacheControlToAnthropic(t *testing.T) {
	t.Run("nil cache control", func(t *testing.T) {
		result := convertToAnthropicCacheControl(nil)
		require.Nil(t, result)
	})

	t.Run("cache control with type only", func(t *testing.T) {
		cc := &llm.CacheControl{
			Type: "ephemeral",
		}
		result := convertToAnthropicCacheControl(cc)
		require.NotNil(t, result)
		require.Equal(t, "ephemeral", result.Type)
		require.Equal(t, "", result.TTL)
	})

	t.Run("cache control with type and ttl", func(t *testing.T) {
		cc := &llm.CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		}
		result := convertToAnthropicCacheControl(cc)
		require.NotNil(t, result)
		require.Equal(t, "ephemeral", result.Type)
		require.Equal(t, "1h", result.TTL)
	})
}

func TestInboundTransformer_CacheControl(t *testing.T) {
	transformer := NewInboundTransformer()

	t.Run("system message with cache control", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"system": [
					{
						"type": "text",
						"text": "You are a helpful assistant.",
						"cache_control": {
							"type": "ephemeral"
						}
					},
					{
						"type": "text",
						"text": "Be professional.",
						"cache_control": {
							"type": "ephemeral",
							"ttl": "5m"
						}
					}
				],
				"messages": [
					{
						"role": "user",
						"content": "Hello!"
					}
				]
			}`),
		}

		result, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check system messages have cache control
		systemMsgs := 0

		for _, msg := range result.Messages {
			if msg.Role == "system" {
				systemMsgs++

				require.NotNil(t, msg.CacheControl, "system message should have cache control")
				require.Equal(t, "ephemeral", msg.CacheControl.Type)

				if systemMsgs == 2 {
					require.Equal(t, "5m", msg.CacheControl.TTL)
				}
			}
		}

		require.Equal(t, 2, systemMsgs, "should have 2 system messages")
	})

	t.Run("message content with cache control", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{
								"type": "text",
								"text": "What is the weather?",
								"cache_control": {
									"type": "ephemeral"
								}
							},
							{
								"type": "text",
								"text": "I need to know the temperature."
							}
						]
					}
				]
			}`),
		}

		result, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Messages, 1)

		msg := result.Messages[0]
		require.Len(t, msg.Content.MultipleContent, 2)

		contentPart := msg.Content.MultipleContent[0]
		require.NotNil(t, contentPart.CacheControl, "content part should have cache control")
		require.Equal(t, "ephemeral", contentPart.CacheControl.Type)

		// Second part should not have cache control
		require.Nil(t, msg.Content.MultipleContent[1].CacheControl)
	})

	t.Run("tools with cache control", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": "Get weather"
					}
				],
				"tools": [
					{
						"name": "get_weather",
						"description": "Get weather",
						"input_schema": {
							"type": "object",
							"properties": {}
						},
						"cache_control": {
							"type": "ephemeral",
							"ttl": "1h"
						}
					}
				]
			}`),
		}

		result, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Tools, 1)

		tool := result.Tools[0]
		require.NotNil(t, tool.CacheControl, "tool should have cache control")
		require.Equal(t, "ephemeral", tool.CacheControl.Type)
		require.Equal(t, "1h", tool.CacheControl.TTL)
	})

	t.Run("tool result with cache control", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{
								"type": "tool_result",
								"tool_use_id": "tool_123",
								"content": "Result data",
								"cache_control": {
									"type": "ephemeral"
								}
							}
						]
					}
				]
			}`),
		}

		result, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Tool result becomes a separate tool message
		var toolMsg *llm.Message

		for i := range result.Messages {
			if result.Messages[i].Role == "tool" {
				toolMsg = &result.Messages[i]
				break
			}
		}

		require.NotNil(t, toolMsg, "should have tool message")
		require.NotNil(t, toolMsg.CacheControl, "tool message should have cache control")
		require.Equal(t, "ephemeral", toolMsg.CacheControl.Type)
	})

	t.Run("tool_use with cache control", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "assistant",
						"content": [
							{
								"type": "tool_use",
								"id": "tool_123",
								"name": "get_weather",
								"input": {"location": "SF"},
								"cache_control": {
									"type": "ephemeral"
								}
							}
						]
					}
				]
			}`),
		}

		result, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Messages, 1)

		msg := result.Messages[0]
		require.Len(t, msg.ToolCalls, 1)

		toolCall := msg.ToolCalls[0]
		require.NotNil(t, toolCall.CacheControl, "tool call should have cache control")
		require.Equal(t, "ephemeral", toolCall.CacheControl.Type)
	})

	t.Run("image content with cache control", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{
								"type": "image",
								"source": {
									"type": "base64",
									"media_type": "image/png",
									"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
								},
								"cache_control": {
									"type": "ephemeral",
									"ttl": "5m"
								}
							}
						]
					}
				]
			}`),
		}

		result, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Messages, 1)

		msg := result.Messages[0]
		require.Len(t, msg.Content.MultipleContent, 1)

		contentPart := msg.Content.MultipleContent[0]
		require.Equal(t, "image_url", contentPart.Type)
		require.NotNil(t, contentPart.CacheControl, "image content part should have cache control")
		require.Equal(t, "ephemeral", contentPart.CacheControl.Type)
		require.Equal(t, "5m", contentPart.CacheControl.TTL)
	})
}

func TestOutboundTransformer_CacheControl(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	t.Run("system message with cache control", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: func() *string { s := "You are helpful"; return &s }(),
					},
					CacheControl: &llm.CacheControl{
						Type: "ephemeral",
						TTL:  "5m",
					},
				},
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: func() *string { s := "Be professional"; return &s }(),
					},
					CacheControl: &llm.CacheControl{
						Type: "ephemeral",
					},
				},
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: func() *string { s := "Hello"; return &s }(),
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		require.NotNil(t, anthropicReq.System)
		require.Len(t, anthropicReq.System.MultiplePrompts, 2)

		// strict mode 下仅保留 system 最后一个结构锚点
		require.Nil(t, anthropicReq.System.MultiplePrompts[0].CacheControl)

		// Check second system prompt has cache control
		require.NotNil(t, anthropicReq.System.MultiplePrompts[1].CacheControl)
		require.Equal(t, "ephemeral", anthropicReq.System.MultiplePrompts[1].CacheControl.Type)
	})

	t.Run("message content with cache control", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "text",
								Text: func() *string { s := "What is the weather?"; return &s }(),
								CacheControl: &llm.CacheControl{
									Type: "ephemeral",
								},
							},
						},
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		require.Len(t, anthropicReq.Messages, 1)
		require.Len(t, anthropicReq.Messages[0].Content.MultipleContent, 1)

		block := anthropicReq.Messages[0].Content.MultipleContent[0]
		require.NotNil(t, block.CacheControl)
		require.Equal(t, "ephemeral", block.CacheControl.Type)
	})

	t.Run("tools with cache control", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: func() *string { s := "Get weather"; return &s }(),
					},
				},
			},
			Tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:        "get_weather",
						Description: "Get weather",
						Parameters:  json.RawMessage(`{"type":"object"}`),
					},
					CacheControl: &llm.CacheControl{
						Type: "ephemeral",
						TTL:  "1h",
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		require.Len(t, anthropicReq.Tools, 1)
		require.NotNil(t, anthropicReq.Tools[0].CacheControl)
		require.Equal(t, "ephemeral", anthropicReq.Tools[0].CacheControl.Type)
		require.Empty(t, anthropicReq.Tools[0].CacheControl.TTL)
	})

	t.Run("tool result with cache control", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role:       "tool",
					ToolCallID: func() *string { s := "tool_123"; return &s }(),
					Content: llm.MessageContent{
						Content: func() *string { s := "Result data"; return &s }(),
					},
					CacheControl: &llm.CacheControl{
						Type: "ephemeral",
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		require.Len(t, anthropicReq.Messages, 1)
		require.Len(t, anthropicReq.Messages[0].Content.MultipleContent, 1)

		block := anthropicReq.Messages[0].Content.MultipleContent[0]
		require.Equal(t, "tool_result", block.Type)
		require.NotNil(t, block.CacheControl)
		require.Equal(t, "ephemeral", block.CacheControl.Type)
	})

	t.Run("image content with cache control", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: "data:image/png;base64,iVBORw0KGgo=",
								},
								CacheControl: &llm.CacheControl{
									Type: "ephemeral",
									TTL:  "5m",
								},
							},
						},
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		require.Len(t, anthropicReq.Messages, 1)
		require.Len(t, anthropicReq.Messages[0].Content.MultipleContent, 1)

		block := anthropicReq.Messages[0].Content.MultipleContent[0]
		require.Equal(t, "image", block.Type)
		require.NotNil(t, block.CacheControl)
		require.Equal(t, "ephemeral", block.CacheControl.Type)
		require.Empty(t, block.CacheControl.TTL)
	})
}

// TestInboundTransformer_TopLevelCacheControl verifies that the top-level
// `cache_control` field (Anthropic's automatic prompt caching) sent by clients
// is captured during inbound parsing and propagated via TransformerMetadata.
func TestInboundTransformer_TopLevelCacheControl(t *testing.T) {
	transformer := NewInboundTransformer()

	t.Run("type only", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-opus-4-6",
				"max_tokens": 22280,
				"cache_control": {"type": "ephemeral"},
				"messages": [{"role": "user", "content": "Hi"}]
			}`),
		}

		got, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.TransformerMetadata)

		cc, ok := got.TransformerMetadata[TransformerMetadataKeyCacheControl].(*CacheControl)
		require.True(t, ok, "cache_control should be stored as *CacheControl in TransformerMetadata")
		require.Equal(t, "ephemeral", cc.Type)
		require.Empty(t, cc.TTL)
	})

	t.Run("with ttl", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{
				"model": "claude-opus-4-6",
				"max_tokens": 22280,
				"cache_control": {"type": "ephemeral", "ttl": "1h"},
				"messages": [{"role": "user", "content": "Hi"}]
			}`),
		}

		got, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, got)

		cc, ok := got.TransformerMetadata[TransformerMetadataKeyCacheControl].(*CacheControl)
		require.True(t, ok)
		require.Equal(t, "ephemeral", cc.Type)
		require.Equal(t, "1h", cc.TTL)
	})
}

// TestOutboundTransformer_TopLevelCacheControlPassthrough verifies that the
// top-level `cache_control` marker is forwarded to the upstream Anthropic
// request as-is and that AxonHub's per-block breakpoint optimization pipeline
// is intentionally skipped (Anthropic manages breakpoints automatically).
func TestOutboundTransformer_TopLevelCacheControlPassthrough(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	systemPrompt := "You are a helpful assistant."

	req := &llm.Request{
		Model:     "claude-opus-4-6",
		MaxTokens: func() *int64 { v := int64(22280); return &v }(),
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: llm.MessageContent{Content: &systemPrompt},
			},
			{
				Role:    "user",
				Content: llm.MessageContent{Content: func() *string { s := "Hi"; return &s }()},
			},
		},
		TransformerMetadata: map[string]any{
			TransformerMetadataKeyCacheControl: &CacheControl{Type: "ephemeral", TTL: "1h"},
		},
	}

	httpReq, err := transformer.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Top-level cache_control must be forwarded to the upstream payload as-is.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(httpReq.Body, &raw))
	rawCC, hasTopLevel := raw["cache_control"]
	require.True(t, hasTopLevel, "top-level cache_control must be forwarded to upstream")

	rawCCMap, ok := rawCC.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ephemeral", rawCCMap["type"])
	require.Equal(t, "1h", rawCCMap["ttl"])

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(httpReq.Body, &anthropicReq))

	// The breakpoint optimization pipeline must NOT run when automatic caching
	// is requested via the top-level field: the system prompt should remain a
	// plain string and no per-block cache_control should be injected.
	require.NotNil(t, anthropicReq.System)
	require.Nil(t, anthropicReq.System.MultiplePrompts)
	require.NotNil(t, anthropicReq.System.Prompt)
	require.Equal(t, systemPrompt, *anthropicReq.System.Prompt)
	require.Equal(t, 0, countCacheControls(&anthropicReq))
}

// TestAnthropicInboundOutbound_TopLevelCacheControlEndToEnd verifies the
// regression reported in https://github.com/looplj/axonhub/issues/1484:
// Anthropic-in & Anthropic-out flows used to drop the top-level
// `cache_control` field. After the fix, the field is preserved end-to-end and
// the breakpoint optimization pipeline is bypassed so Anthropic's automatic
// caching behavior remains intact.
func TestAnthropicInboundOutbound_TopLevelCacheControlEndToEnd(t *testing.T) {
	inbound := NewInboundTransformer()
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{
			"model": "claude-opus-4-6",
			"max_tokens": 22280,
			"top_p": 0.95,
			"stream": false,
			"temperature": 1,
			"cache_control": {"type": "ephemeral"},
			"messages": [{"role": "user", "content": "Hi"}]
		}`),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	upstream, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.NotNil(t, upstream)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(upstream.Body, &raw))

	// The top-level cache_control marker must be forwarded as-is.
	rawCC, hasTopLevel := raw["cache_control"]
	require.True(t, hasTopLevel, "top-level cache_control must be forwarded to upstream")

	rawCCMap, ok := rawCC.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ephemeral", rawCCMap["type"])

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(upstream.Body, &anthropicReq))

	// Optimization pipeline must be skipped: the user message content should
	// remain a plain string with no injected per-block cache_control.
	require.Len(t, anthropicReq.Messages, 1)
	require.NotNil(t, anthropicReq.Messages[0].Content.Content)
	require.Equal(t, "Hi", *anthropicReq.Messages[0].Content.Content)
	require.Equal(t, 0, countCacheControls(&anthropicReq))
}
