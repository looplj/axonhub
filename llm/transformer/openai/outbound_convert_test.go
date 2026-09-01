package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestRequestFromLLM(t *testing.T) {
	tests := []struct {
		name     string
		llmReq   *llm.Request
		validate func(*testing.T, *Request)
	}{
		{
			name:   "nil request",
			llmReq: nil,
			validate: func(t *testing.T, req *Request) {
				require.Nil(t, req)
			},
		},
		{
			name: "basic request",
			llmReq: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello there!"),
						},
					},
				},
				Stream: lo.ToPtr(true),
			},
			validate: func(t *testing.T, req *Request) {
				require.NotNil(t, req)
				require.Equal(t, "gpt-4", req.Model)
				require.Len(t, req.Messages, 1)
				require.Equal(t, "assistant", req.Messages[0].Role)
				require.True(t, *req.Stream)
			},
		},
		{
			name: "request with helper fields stripped",
			llmReq: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role:         "tool",
						ToolCallID:   lo.ToPtr("call_123"),
						MessageIndex: lo.ToPtr(1), // Helper field - should not be in OpenAI model
						Content:      llm.MessageContent{Content: lo.ToPtr("result")},
					},
				},
				APIFormat: llm.APIFormatOpenAIChatCompletion, // Helper field
			},
			validate: func(t *testing.T, req *Request) {
				require.NotNil(t, req)
				require.Equal(t, "call_123", *req.Messages[0].ToolCallID)
				// OpenAI Request doesn't have MessageIndex or RawAPIFormat fields
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RequestFromLLM(context.Background(), tt.llmReq, ReasoningFieldNone)
			tt.validate(t, result)
		})
	}
}

func TestRequestFromLLM_FiltersResponsesCustomTools(t *testing.T) {
	req := RequestFromLLM(context.Background(), &llm.Request{
		Model:    "gpt-4o",
		Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
		Tools: []llm.Tool{
			{
				Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomTool: &llm.ResponseCustomTool{
					Name: "apply_patch",
				},
			},
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name:       "get_weather",
					Parameters: []byte(`{"type":"object"}`),
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Tools, 1)
	require.Equal(t, llm.ToolTypeFunction, req.Tools[0].Type)
}

func TestMessageContentPartAudioRoundTrip(t *testing.T) {
	part := llm.MessageContentPart{
		Type: "input_audio",
		InputAudio: &llm.InputAudio{
			Format: "mp3",
			Data:   "audio-base64",
		},
	}

	oaiPart := MessageContentPartFromLLM(part)
	require.Equal(t, "input_audio", oaiPart.Type)
	require.NotNil(t, oaiPart.InputAudio)
	require.Equal(t, "mp3", oaiPart.InputAudio.Format)
	require.Equal(t, "audio-base64", oaiPart.InputAudio.Data)

	roundTrip := oaiPart.ToLLMPart()
	require.Equal(t, "input_audio", roundTrip.Type)
	require.NotNil(t, roundTrip.InputAudio)
	require.Equal(t, "mp3", roundTrip.InputAudio.Format)
	require.Equal(t, "audio-base64", roundTrip.InputAudio.Data)
}

func TestMessageContentFromLLM_IgnoresCompactionParts(t *testing.T) {
	content := MessageContentFromLLM(llm.MessageContent{
		MultipleContent: []llm.MessageContentPart{
			{
				Type: "compaction",
				Compact: &llm.CompactContent{
					ID:               "cmp_123",
					EncryptedContent: "secret",
				},
			},
			{
				Type: "compaction_summary",
				Compact: &llm.CompactContent{
					ID:               "cmp_456",
					EncryptedContent: "summary",
				},
			},
			{
				Type: "text",
				Text: lo.ToPtr("visible"),
			},
		},
	})

	require.Len(t, content.MultipleContent, 1)
	require.Equal(t, "text", content.MultipleContent[0].Type)
	require.NotNil(t, content.MultipleContent[0].Text)
	require.Equal(t, "visible", *content.MultipleContent[0].Text)
}

func TestRequestFromLLM_IgnoresCompactionPartsInMessages(t *testing.T) {
	req := RequestFromLLM(context.Background(), &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "cmp_123",
								EncryptedContent: "secret",
							},
						},
						{
							Type: "compaction_summary",
							Compact: &llm.CompactContent{
								ID:               "cmp_456",
								EncryptedContent: "summary",
							},
						},
						{
							Type: "text",
							Text: lo.ToPtr("hello"),
						},
					},
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].Content.MultipleContent, 1)
	require.Equal(t, "text", req.Messages[0].Content.MultipleContent[0].Type)
	require.NotNil(t, req.Messages[0].Content.MultipleContent[0].Text)
	require.Equal(t, "hello", *req.Messages[0].Content.MultipleContent[0].Text)
}

func TestMessageAudioRoundTrip(t *testing.T) {
	msg := llm.Message{
		Role: "assistant",
		Content: llm.MessageContent{
			Content: lo.ToPtr("Audio reply"),
		},
		Audio: &llm.OutputAudio{
			ID:         "audio_123",
			Data:       "base64-audio",
			ExpiresAt:  1234567890,
			Transcript: "hello world",
		},
	}

	oaiMsg := MessageFromLLM(msg)
	require.NotNil(t, oaiMsg.Audio)
	require.Equal(t, "audio_123", oaiMsg.Audio.ID)
	require.Equal(t, "base64-audio", oaiMsg.Audio.Data)
	require.Equal(t, int64(1234567890), oaiMsg.Audio.ExpiresAt)
	require.Equal(t, "hello world", oaiMsg.Audio.Transcript)

	roundTrip := oaiMsg.ToLLMMessage()
	require.NotNil(t, roundTrip.Audio)
	require.Equal(t, "audio_123", roundTrip.Audio.ID)
	require.Equal(t, "base64-audio", roundTrip.Audio.Data)
	require.Equal(t, int64(1234567890), roundTrip.Audio.ExpiresAt)
	require.Equal(t, "hello world", roundTrip.Audio.Transcript)
}

func TestResponse_ToLLMResponse(t *testing.T) {
	tests := []struct {
		name     string
		oaiResp  *Response
		validate func(*testing.T, *llm.Response)
	}{
		{
			name:    "nil response",
			oaiResp: nil,
			validate: func(t *testing.T, resp *llm.Response) {
				require.Nil(t, resp)
			},
		},
		{
			name: "basic response",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chatcmpl-123", resp.ID)
				require.Equal(t, "chat.completion", resp.Object)
				require.Len(t, resp.Choices, 1)
				require.Equal(t, "Hello!", *resp.Choices[0].Message.Content.Content)
				require.Equal(t, "stop", *resp.Choices[0].FinishReason)
			},
		},
		{
			name: "streaming response with delta",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Delta: &Message{
							Content: MessageContent{Content: lo.ToPtr("chunk")},
						},
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "chat.completion.chunk", resp.Object)
				require.NotNil(t, resp.Choices[0].Delta)
				require.Equal(t, "chunk", *resp.Choices[0].Delta.Content.Content)
			},
		},
		{
			name: "response with usage",
			oaiResp: &Response{
				ID:     "chatcmpl-123",
				Object: "chat.completion",
				Model:  "gpt-4",
				Choices: []Choice{
					{Index: 0, Message: &Message{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("Hi")}}},
				},
				Usage: &Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(10), resp.Usage.PromptTokens)
				require.Equal(t, int64(5), resp.Usage.CompletionTokens)
				require.Equal(t, int64(15), resp.Usage.TotalTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiResp.ToLLMResponse()
			tt.validate(t, result)
		})
	}
}

func TestMessage_ToLLMMessage_WithAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		oaiMsg   Message
		validate func(*testing.T, llm.Message)
	}{
		{
			name: "message with annotations",
			oaiMsg: Message{
				Role:    "assistant",
				Content: MessageContent{Content: lo.ToPtr("The meaning of life...")},
				Annotations: []Annotation{
					{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(0)),
						EndIndex:   lo.ToPtr(int64(11)),
						URLCitation: &URLCitation{
							URL:   "https://en.wikipedia.org/wiki/Meaning_of_life",
							Title: "Meaning of life - Wikipedia",
						},
					},
					{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(20)),
						EndIndex:   lo.ToPtr(int64(27)),
						URLCitation: &URLCitation{
							URL:   "https://plato.stanford.edu/entries/life-meaning/",
							Title: "The Meaning of Life - Stanford Encyclopedia",
						},
					},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Len(t, msg.Annotations, 2)
				require.Equal(t, "url_citation", msg.Annotations[0].Type)
				require.NotNil(t, msg.Annotations[0].StartIndex)
				require.Equal(t, int64(0), *msg.Annotations[0].StartIndex)
				require.NotNil(t, msg.Annotations[0].EndIndex)
				require.Equal(t, int64(11), *msg.Annotations[0].EndIndex)
				require.NotNil(t, msg.Annotations[0].URLCitation)
				require.Equal(t, "https://en.wikipedia.org/wiki/Meaning_of_life", msg.Annotations[0].URLCitation.URL)
				require.Equal(t, "Meaning of life - Wikipedia", msg.Annotations[0].URLCitation.Title)
				require.NotNil(t, msg.Annotations[1].StartIndex)
				require.Equal(t, int64(20), *msg.Annotations[1].StartIndex)
				require.NotNil(t, msg.Annotations[1].EndIndex)
				require.Equal(t, int64(27), *msg.Annotations[1].EndIndex)
			},
		},
		{
			name: "message without annotations",
			oaiMsg: Message{
				Role:    "assistant",
				Content: MessageContent{Content: lo.ToPtr("Hello!")},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Nil(t, msg.Annotations)
			},
		},
		{
			name: "message with empty annotations",
			oaiMsg: Message{
				Role:        "assistant",
				Content:     MessageContent{Content: lo.ToPtr("Hello!")},
				Annotations: []Annotation{},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Nil(t, msg.Annotations)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiMsg.ToLLMMessage()
			tt.validate(t, result)
		})
	}
}

func TestResponse_ToLLMResponse_WithCitations(t *testing.T) {
	tests := []struct {
		name     string
		oaiResp  *Response
		validate func(*testing.T, *llm.Response)
	}{
		{
			name: "response with citations",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "llama-3.1-sonar-small-128k-online",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("The meaning of life is...")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Citations: []string{
					"https://www.theatlantic.com/family/archive/2021/10/meaning-life-macronutrients-purpose-search/620440/",
					"https://en.wikipedia.org/wiki/Meaning_of_life",
					"https://greatergood.berkeley.edu/article/item/three_ways_to_see_meaning_in_your_life",
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.TransformerMetadata)
				citations, ok := resp.TransformerMetadata[TransformerMetadataKeyCitations].([]string)
				require.True(t, ok)
				require.Len(t, citations, 3)
				require.Contains(t, citations, "https://www.theatlantic.com/family/archive/2021/10/meaning-life-macronutrients-purpose-search/620440/")
				require.Contains(t, citations, "https://en.wikipedia.org/wiki/Meaning_of_life")
				require.Contains(t, citations, "https://greatergood.berkeley.edu/article/item/three_ways_to_see_meaning_in_your_life")
			},
		},
		{
			name: "response without citations",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				// TransformerMetadata should be nil when no citations
				require.Nil(t, resp.TransformerMetadata)
			},
		},
		{
			name: "response with empty citations",
			oaiResp: &Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: &Message{
							Role:    "assistant",
							Content: MessageContent{Content: lo.ToPtr("Hello!")},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Citations: []string{},
			},
			validate: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				// TransformerMetadata should be nil when citations are empty
				require.Nil(t, resp.TransformerMetadata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.oaiResp.ToLLMResponse()
			tt.validate(t, result)
		})
	}
}

func TestRequestFromLLM_KeepsGoogleThoughtSignatureInRequestModel(t *testing.T) {
	req := RequestFromLLM(context.Background(), &llm.Request{
		Model: "gemini-3-pro",
		Messages: []llm.Message{
			{
				Role:               "assistant",
				ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("sig_from_reasoning")),
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
						Index: 0,
						TransformerMetadata: map[string]any{
							TransformerMetadataKeyGoogleThoughtSignature: "sig_from_metadata",
						},
					},
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].ToolCalls, 1)
	require.NotNil(t, req.Messages[0].ToolCalls[0].ExtraContent)
	require.NotNil(t, req.Messages[0].ToolCalls[0].ExtraContent.Google)
	require.Equal(t, "sig_from_metadata", req.Messages[0].ToolCalls[0].ExtraContent.Google.ThoughtSignature)
}

func TestMessageFromLLM_DoesNotOverrideFirstToolCallWhenMetadataExists(t *testing.T) {
	msg := MessageFromLLM(llm.Message{
		Role:               "assistant",
		ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("sig_from_second_tool_call")),
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "tool_a",
					Arguments: "{}",
				},
				Index: 0,
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "tool_b",
					Arguments: "{}",
				},
				Index: 1,
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyGoogleThoughtSignature: "sig_from_second_tool_call",
				},
			},
		},
	})

	require.Len(t, msg.ToolCalls, 2)
	require.Nil(t, msg.ToolCalls[0].ExtraContent)
	require.NotNil(t, msg.ToolCalls[1].ExtraContent)
	require.NotNil(t, msg.ToolCalls[1].ExtraContent.Google)
	require.Equal(t, "sig_from_second_tool_call", msg.ToolCalls[1].ExtraContent.Google.ThoughtSignature)
}

func TestMessageFromLLM_GeminiReasoningSignatureDoesNotInjectThoughtSignature(t *testing.T) {
	msg := MessageFromLLM(llm.Message{
		Role:               "assistant",
		ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("gemini_signature")),
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "tool_a",
					Arguments: "{}",
				},
				Index: 0,
			},
		},
	})

	require.Len(t, msg.ToolCalls, 1)
	require.Nil(t, msg.ToolCalls[0].ExtraContent)
}

func TestApplyReasoningEffortMapping(t *testing.T) {
	tests := []struct {
		name    string
		effort  string
		mapping []llm.ReasoningEffortMapping
		want    string
	}{
		{name: "xhigh mapped to max", effort: "xhigh", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: "max"},
		{name: "high mapped to medium", effort: "high", mapping: []llm.ReasoningEffortMapping{{From: "high", To: "medium"}}, want: "medium"},
		{name: "max passes through (not in list)", effort: "max", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: "max"},
		{name: "low passes through (not in list)", effort: "low", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: "low"},
		{name: "empty effort passes through", effort: "", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}}, want: ""},
		{name: "nil mapping passes through", effort: "xhigh", mapping: nil, want: "xhigh"},
		{name: "empty mapping passes through", effort: "xhigh", mapping: []llm.ReasoningEffortMapping{}, want: "xhigh"},
		{name: "multiple entries hit", effort: "high", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}, {From: "high", To: "medium"}}, want: "medium"},
		{name: "multiple entries miss", effort: "low", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}, {From: "high", To: "medium"}}, want: "low"},
		{name: "first matching entry wins", effort: "xhigh", mapping: []llm.ReasoningEffortMapping{{From: "xhigh", To: "max"}, {From: "xhigh", To: "high"}}, want: "max"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, applyReasoningEffortMapping(tt.effort, tt.mapping))
		})
	}
}

// TestRequestFromLLM_PreservesReasoningEffort ensures RequestFromLLM does NOT map
// reasoning_effort: mapping is the OutboundTransformer's responsibility (driven by
// Config.ReasoningEffortMapping), not the package-level converter's.
func TestRequestFromLLM_PreservesReasoningEffort(t *testing.T) {
	req := RequestFromLLM(context.Background(), &llm.Request{
		Model:           "gpt-4",
		ReasoningEffort: "xhigh",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Equal(t, "xhigh", req.ReasoningEffort, "RequestFromLLM must keep reasoning_effort unchanged; mapping happens in TransformRequest")
}

// Assistant messages that carry tool calls but no content must still serialize a
// content field. Omitting it (nil content) or emitting null (all parts filtered
// out) is accepted by OpenAI but rejected by stricter OpenAI-compatible upstreams
// whose schema only allows a string or an array.
func TestMessageFromLLM_ToolCallOnlyMessageKeepsContentField(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "shell_command",
				Arguments: `{"command":"ls"}`,
			},
		},
	}

	tests := []struct {
		name    string
		message llm.Message
	}{
		{
			name: "no content at all",
			message: llm.Message{
				Role:      "assistant",
				ToolCalls: toolCalls,
			},
		},
		{
			name: "every content part filtered out",
			message: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type:    "compaction",
							Compact: &llm.CompactContent{ID: "cmp_1", EncryptedContent: "secret"},
						},
					},
				},
				ToolCalls: toolCalls,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := MessageFromLLM(tt.message)

			data, err := json.Marshal(msg)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(data, &decoded))

			content, ok := decoded["content"]
			require.True(t, ok, "content field must be present, got %s", data)
			require.NotNil(t, content, "content must not be null, got %s", data)
			require.Equal(t, "", content)
		})
	}
}

// Content that survives conversion must be preserved as-is.
func TestMessageFromLLM_ToolCallMessageKeepsExistingContent(t *testing.T) {
	msg := MessageFromLLM(llm.Message{
		Role:    "assistant",
		Content: llm.MessageContent{Content: lo.ToPtr("calling a tool")},
		ToolCalls: []llm.ToolCall{
			{
				ID:       "call_1",
				Type:     "function",
				Function: llm.FunctionCall{Name: "shell_command", Arguments: "{}"},
			},
		},
	})

	require.NotNil(t, msg.Content.Content)
	require.Equal(t, "calling a tool", *msg.Content.Content)
}

// Messages without tool calls keep their existing serialization, so the
// normalization above cannot change unrelated payloads.
func TestMessageFromLLM_WithoutToolCallsContentUnchanged(t *testing.T) {
	msg := MessageFromLLM(llm.Message{Role: "user"})

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	_, ok := decoded["content"]
	require.False(t, ok, "content must stay omitted for messages without tool calls, got %s", data)
}

// Responses splits text parts into input_text/output_text, but Chat Completions
// only knows "text". Types it does understand must not be rewritten.
func TestMessageContentPartFromLLM_NormalizesTextPartTypes(t *testing.T) {
	tests := []struct {
		name     string
		partType string
		expected string
	}{
		{name: "input_text becomes text", partType: "input_text", expected: "text"},
		{name: "output_text becomes text", partType: "output_text", expected: "text"},
		{name: "text is unchanged", partType: "text", expected: "text"},
		{name: "image_url is unchanged", partType: "image_url", expected: "image_url"},
		{name: "video_url is unchanged", partType: "video_url", expected: "video_url"},
		{name: "input_audio is unchanged", partType: "input_audio", expected: "input_audio"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part := MessageContentPartFromLLM(llm.MessageContentPart{
				Type: tt.partType,
				Text: lo.ToPtr("hello"),
			})

			require.Equal(t, tt.expected, part.Type)
		})
	}
}

// Multi-part content is the path where Responses text types actually reach an
// upstream: a lone text part is collapsed into a plain string before this point.
func TestRequestFromLLM_NormalizesTextPartTypesInMessages(t *testing.T) {
	req := RequestFromLLM(context.Background(), &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "input_text", Text: lo.ToPtr("describe this")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/a.png"}},
					},
				},
			},
		},
	}, ReasoningFieldNone)

	require.NotNil(t, req)
	require.Len(t, req.Messages, 1)
	require.Len(t, req.Messages[0].Content.MultipleContent, 2)
	require.Equal(t, "text", req.Messages[0].Content.MultipleContent[0].Type)
	require.Equal(t, "image_url", req.Messages[0].Content.MultipleContent[1].Type)
}
