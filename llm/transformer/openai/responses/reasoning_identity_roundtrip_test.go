package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestResponsesEncryptedReasoningIdentitySurvivesConversationReplay(t *testing.T) {
	providerTransformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	clientTransformer := NewInboundTransformer()

	providerResponse := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"id":"resp_original",
			"object":"response",
			"created_at":1784548800,
			"status":"completed",
			"model":"gpt-5.6",
			"output":[
				{
					"id":"rs_original_from_provider",
					"type":"reasoning",
					"status":"completed",
					"summary":[{"type":"summary_text","text":"checked the repository"}],
					"encrypted_content":"opaque_provider_ciphertext"
				},
				{
					"id":"msg_original_from_provider",
					"type":"message",
					"status":"completed",
					"role":"assistant",
					"content":[{"type":"output_text","text":"Done.","annotations":[]}]
				}
			],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`),
	}

	canonicalResponse, err := providerTransformer.TransformResponse(t.Context(), providerResponse)
	require.NoError(t, err)
	require.Len(t, canonicalResponse.Choices, 1)
	require.NotNil(t, canonicalResponse.Choices[0].Message)
	require.Nil(t, canonicalResponse.Choices[0].Message.ResponseReasoningItemID,
		"response output identity belongs to the Responses-native response sidecar, not the request identity carrier")
	require.NotNil(t, canonicalResponse.ProviderExtensions)
	require.NotNil(t, canonicalResponse.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, canonicalResponse.ProviderExtensions.OpenAIResponses.Response)
	require.Len(t, canonicalResponse.ProviderExtensions.OpenAIResponses.Response.RawOutputItems, 1)
	require.Equal(t, "reasoning", canonicalResponse.ProviderExtensions.OpenAIResponses.Response.RawOutputItems[0].Type)

	clientResponse, err := clientTransformer.TransformResponse(t.Context(), canonicalResponse)
	require.NoError(t, err)

	var savedResponse Response
	require.NoError(t, json.Unmarshal(clientResponse.Body, &savedResponse))
	require.Len(t, savedResponse.Output, 2)
	require.Equal(t, "reasoning", savedResponse.Output[0].Type)
	require.Equal(t, "rs_original_from_provider", savedResponse.Output[0].ID)
	require.Equal(t, "opaque_provider_ciphertext", requireString(t, savedResponse.Output[0].EncryptedContent))

	continuationBody, err := json.Marshal(Request{
		Model: "gpt-5.6",
		Input: Input{Items: append(savedResponse.Output, Item{
			Type: "message",
			Role: "user",
			Content: &Input{Items: []Item{{
				Type: "input_text",
				Text: lo.ToPtr("continue"),
			}}},
		})},
	})
	require.NoError(t, err)

	continuation, err := clientTransformer.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    continuationBody,
	})
	require.NoError(t, err)

	providerRequest, err := providerTransformer.TransformRequest(t.Context(), continuation)
	require.NoError(t, err)

	var replayed Request
	require.NoError(t, json.Unmarshal(providerRequest.Body, &replayed))
	require.NotEmpty(t, replayed.Input.Items)
	require.Equal(t, "reasoning", replayed.Input.Items[0].Type)
	require.Equal(t, "rs_original_from_provider", replayed.Input.Items[0].ID)
	require.Equal(t, "opaque_provider_ciphertext", requireString(t, replayed.Input.Items[0].EncryptedContent))
}

func TestResponsesResponseDoesNotUseRequestReasoningIdentityCarrier(t *testing.T) {
	clientTransformer := NewInboundTransformer()
	clientResponse, err := clientTransformer.TransformResponse(t.Context(), &llm.Response{
		ID:      "resp_canonical",
		Object:  "chat.completion",
		Created: 1784548800,
		Model:   "gpt-5.6",
		Choices: []llm.Choice{{
			Index: 0,
			Message: &llm.Message{
				ID:                      "msg_canonical",
				Role:                    "assistant",
				Content:                 llm.MessageContent{Content: lo.ToPtr("Done.")},
				ResponseReasoningItemID: lo.ToPtr("rs_request_input_only"),
			},
		}},
	})
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(clientResponse.Body, &response))
	require.Equal(t, []string{"message"}, itemTypes(response.Output),
		"request input identity must not create or identify a response output item")
}

func TestResponsesMultipleEncryptedReasoningItemsSurviveClientResponseReplay(t *testing.T) {
	providerTransformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	clientTransformer := NewInboundTransformer()

	canonicalResponse, err := providerTransformer.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"id":"resp_multiple_reasoning",
			"object":"response",
			"created_at":1784548800,
			"status":"completed",
			"model":"gpt-5.6",
			"output":[
				{"id":"rs_first","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"first"}],"encrypted_content":"gAAAA_first"},
				{"id":"fc_between","type":"function_call","status":"completed","call_id":"call_between","name":"read","arguments":"{}"},
				{"id":"rs_second","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"second"}],"encrypted_content":"gAAAA_second"},
				{"id":"msg_final","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Done.","annotations":[]}]}
			],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`),
	})
	require.NoError(t, err)
	require.Len(t, canonicalResponse.Choices, 1)
	require.NotNil(t, canonicalResponse.Choices[0].Message)
	require.Nil(t, canonicalResponse.Choices[0].Message.ResponseReasoningItemID)

	clientResponse, err := clientTransformer.TransformResponse(t.Context(), canonicalResponse)
	require.NoError(t, err)

	var replayed Response
	require.NoError(t, json.Unmarshal(clientResponse.Body, &replayed))
	require.Len(t, replayed.Output, 4)
	require.Equal(t, []string{"reasoning", "function_call", "reasoning", "message"}, itemTypes(replayed.Output))
	require.Equal(t, "rs_first", replayed.Output[0].ID)
	require.Equal(t, "gAAAA_first", requireString(t, replayed.Output[0].EncryptedContent))
	require.Equal(t, "rs_second", replayed.Output[2].ID)
	require.Equal(t, "gAAAA_second", requireString(t, replayed.Output[2].EncryptedContent))

	continuationBody, err := json.Marshal(Request{
		Model: "gpt-5.6",
		Input: Input{Items: append(replayed.Output, Item{
			Type: "message",
			Role: "user",
			Content: &Input{Items: []Item{{
				Type: "input_text",
				Text: lo.ToPtr("continue"),
			}}},
		})},
	})
	require.NoError(t, err)
	canonicalRequest, err := clientTransformer.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    continuationBody,
	})
	require.NoError(t, err)
	providerRequest, err := providerTransformer.TransformRequest(t.Context(), canonicalRequest)
	require.NoError(t, err)

	var continuation Request
	require.NoError(t, json.Unmarshal(providerRequest.Body, &continuation))
	require.Equal(t, []string{"reasoning", "function_call", "reasoning", "message", "message"}, itemTypes(continuation.Input.Items))
	require.Equal(t, "rs_first", continuation.Input.Items[0].ID)
	require.Equal(t, "gAAAA_first", requireString(t, continuation.Input.Items[0].EncryptedContent))
	require.Equal(t, "rs_second", continuation.Input.Items[2].ID)
	require.Equal(t, "gAAAA_second", requireString(t, continuation.Input.Items[2].EncryptedContent))
}

func TestCompactResponseEncryptedReasoningAndCompactionIdentitySurviveRoundTrip(t *testing.T) {
	providerTransformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	clientTransformer := NewCompactInboundTransformer()

	canonicalResponse, err := providerTransformer.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    &httpclient.Request{RequestType: string(llm.RequestTypeCompact)},
		Body: []byte(`{
			"id":"resp_compact_original",
			"object":"response.compaction",
			"created_at":1784548800,
			"model":"gpt-5.6",
			"output":[
				{"id":"rs_compact_original","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"compact reasoning"}],"encrypted_content":"opaque_compact_reasoning"},
				{"id":"msg_compact_original","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Compacted.","annotations":[]}]},
				{"id":"cmp_summary_original","type":"compaction_summary","encrypted_content":"opaque_compaction_summary"}
			]
		}`),
	})
	require.NoError(t, err)
	require.NotNil(t, canonicalResponse.Compact)
	require.NotEmpty(t, canonicalResponse.Compact.Output)
	require.Nil(t, canonicalResponse.Compact.Output[0].ResponseReasoningItemID,
		"compact response output identity also belongs to the native response sidecar")
	require.NotNil(t, canonicalResponse.ProviderExtensions)
	require.NotNil(t, canonicalResponse.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, canonicalResponse.ProviderExtensions.OpenAIResponses.Response)
	require.Len(t, canonicalResponse.ProviderExtensions.OpenAIResponses.Response.RawOutputItems, 1)
	require.Equal(t, "reasoning", canonicalResponse.ProviderExtensions.OpenAIResponses.Response.RawOutputItems[0].Type)

	clientResponse, err := clientTransformer.TransformResponse(t.Context(), canonicalResponse)
	require.NoError(t, err)

	var replayed CompactAPIResponse
	require.NoError(t, json.Unmarshal(clientResponse.Body, &replayed))
	require.Equal(t, []string{"reasoning", "message", "compaction_summary"}, itemTypes(replayed.Output))
	require.Equal(t, "rs_compact_original", replayed.Output[0].ID)
	require.Equal(t, "opaque_compact_reasoning", requireString(t, replayed.Output[0].EncryptedContent))
	require.Equal(t, "msg_compact_original", replayed.Output[1].ID)
	require.Equal(t, "cmp_summary_original", replayed.Output[2].ID)
	require.Equal(t, "opaque_compaction_summary", requireString(t, replayed.Output[2].EncryptedContent))
}

func TestStreamedEncryptedReasoningIdentitySurvivesConversationReplay(t *testing.T) {
	completed := roundTripCompletedResponse(t, []*httpclient.StreamEvent{
		{Type: string(StreamEventTypeResponseCreated), Data: []byte(`{"type":"response.created","response":{"id":"resp_stream_replay","object":"response","created_at":1784548800,"model":"gpt-5.6","status":"in_progress","output":[]}}`)},
		{Type: string(StreamEventTypeOutputItemAdded), Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_stream_original","type":"reasoning","status":"in_progress","summary":[],"encrypted_content":"opaque_stream_provisional"}}`)},
		{Type: string(StreamEventTypeOutputItemDone), Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_stream_original","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"stream reasoning"}],"encrypted_content":"opaque_stream_final"}}`)},
		{Type: string(StreamEventTypeOutputItemAdded), Data: []byte(`{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_stream_original","type":"message","status":"in_progress","role":"assistant","content":[]}}`)},
		{Type: string(StreamEventTypeContentPartAdded), Data: []byte(`{"type":"response.content_part.added","item_id":"msg_stream_original","output_index":1,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`)},
		{Type: string(StreamEventTypeOutputTextDelta), Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_stream_original","output_index":1,"content_index":0,"delta":"Streamed."}`)},
		{Type: string(StreamEventTypeOutputTextDone), Data: []byte(`{"type":"response.output_text.done","item_id":"msg_stream_original","output_index":1,"content_index":0,"text":"Streamed."}`)},
		{Type: string(StreamEventTypeContentPartDone), Data: []byte(`{"type":"response.content_part.done","item_id":"msg_stream_original","output_index":1,"content_index":0,"part":{"type":"output_text","text":"Streamed.","annotations":[]}}`)},
		{Type: string(StreamEventTypeOutputItemDone), Data: []byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"msg_stream_original","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Streamed.","annotations":[]}]}}`)},
		{Type: string(StreamEventTypeResponseCompleted), Data: []byte(`{"type":"response.completed","response":{"id":"resp_stream_replay","object":"response","created_at":1784548800,"model":"gpt-5.6","status":"completed","output":[{"id":"rs_stream_original","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"stream reasoning"}],"encrypted_content":"opaque_stream_final"},{"id":"msg_stream_original","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Streamed.","annotations":[]}]}]}}`)},
	})
	require.Len(t, completed.Output, 2)
	require.Equal(t, "rs_stream_original", completed.Output[0].ID)
	require.Equal(t, "opaque_stream_final", requireString(t, completed.Output[0].EncryptedContent))

	continuationBody, err := json.Marshal(Request{
		Model: "gpt-5.6",
		Input: Input{Items: append(completed.Output, Item{
			Type: "message",
			Role: "user",
			Content: &Input{Items: []Item{{
				Type: "input_text",
				Text: lo.ToPtr("continue"),
			}}},
		})},
	})
	require.NoError(t, err)

	clientTransformer := NewInboundTransformer()
	canonicalRequest, err := clientTransformer.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    continuationBody,
	})
	require.NoError(t, err)

	providerTransformer, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	providerRequest, err := providerTransformer.TransformRequest(t.Context(), canonicalRequest)
	require.NoError(t, err)

	var replayed Request
	require.NoError(t, json.Unmarshal(providerRequest.Body, &replayed))
	require.Equal(t, "reasoning", replayed.Input.Items[0].Type)
	require.Equal(t, "rs_stream_original", replayed.Input.Items[0].ID)
	require.Equal(t, "opaque_stream_final", requireString(t, replayed.Input.Items[0].EncryptedContent))
}

func TestResponsesClientResponseDoesNotExposeCrossProtocolReasoningSignatureAsEncryptedContent(t *testing.T) {
	clientTransformer := NewInboundTransformer()

	clientResponse, err := clientTransformer.TransformResponse(t.Context(), &llm.Response{
		ID:        "anthropic_response",
		Object:    "chat.completion",
		Model:     "claude-sonnet-4-6",
		APIFormat: llm.APIFormatAnthropicMessage,
		Choices: []llm.Choice{{
			Index: 0,
			Message: &llm.Message{
				Role:               "assistant",
				ReasoningContent:   lo.ToPtr("visible Anthropic thinking"),
				ReasoningSignature: lo.ToPtr("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg=="),
				Content:            llm.MessageContent{Content: lo.ToPtr("answer")},
			},
			FinishReason: lo.ToPtr("stop"),
		}},
	})
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(clientResponse.Body, &response))
	reasoning := findItemByType(t, response.Output, "reasoning")
	require.Nil(t, reasoning.EncryptedContent,
		"an Anthropic thinking signature must not be relabeled as OpenAI Responses encrypted_content")
	require.Equal(t, "visible Anthropic thinking", reasoning.Summary[0].Text)
}

func TestResponsesClientResponseDoesNotPairEncryptedContentWithSyntheticID(t *testing.T) {
	clientTransformer := NewInboundTransformer()

	clientResponse, err := clientTransformer.TransformResponse(t.Context(), &llm.Response{
		ID:        "resp_no_provenance",
		Object:    "chat.completion",
		Model:     "o3",
		APIFormat: llm.APIFormatOpenAIResponse,
		// Responses format alone is insufficient: ciphertext without native item
		// provenance must not mint a synthetic rs_* id (item_id mismatch risk).
		Choices: []llm.Choice{{
			Index: 0,
			Message: &llm.Message{
				Role:               "assistant",
				ReasoningContent:   lo.ToPtr("visible summary only"),
				ReasoningSignature: lo.ToPtr("opaque_ciphertext_without_item_id"),
				Content:            llm.MessageContent{Content: lo.ToPtr("answer")},
			},
			FinishReason: lo.ToPtr("stop"),
		}},
	})
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(clientResponse.Body, &response))
	reasoning := findItemByType(t, response.Output, "reasoning")
	require.Nil(t, reasoning.EncryptedContent,
		"ciphertext without Responses-native item id must not be re-emitted")
	require.Equal(t, "visible summary only", reasoning.Summary[0].Text)
}

func TestResponsesClientStreamDoesNotExposeCrossProtocolReasoningSignatureAsEncryptedContent(t *testing.T) {
	clientTransformer := NewInboundTransformer()
	stream, err := clientTransformer.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:        "anthropic_stream",
			Object:    "chat.completion.chunk",
			Model:     "claude-sonnet-4-6",
			APIFormat: llm.APIFormatAnthropicMessage,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role:             "assistant",
					ReasoningContent: lo.ToPtr("visible Anthropic thinking"),
				},
			}},
		},
		{
			ID:        "anthropic_stream",
			Object:    "chat.completion.chunk",
			Model:     "claude-sonnet-4-6",
			APIFormat: llm.APIFormatAnthropicMessage,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ReasoningSignature: lo.ToPtr("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg=="),
				},
			}},
		},
		{
			ID:        "anthropic_stream",
			Object:    "chat.completion.chunk",
			Model:     "claude-sonnet-4-6",
			APIFormat: llm.APIFormatAnthropicMessage,
			Choices: []llm.Choice{{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: lo.ToPtr("stop"),
			}},
		},
		{
			ID:        "anthropic_stream",
			Object:    "chat.completion.chunk",
			Model:     "claude-sonnet-4-6",
			APIFormat: llm.APIFormatAnthropicMessage,
			Usage:     &llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}))
	require.NoError(t, err)

	var reasoningDone *Item
	for stream.Next() {
		event := stream.Current()
		var decoded StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &decoded))
		if decoded.Type == StreamEventTypeOutputItemDone && decoded.Item != nil && decoded.Item.Type == "reasoning" {
			reasoningDone = decoded.Item
		}
	}
	require.NoError(t, stream.Err())
	require.NotNil(t, reasoningDone)
	require.Nil(t, reasoningDone.EncryptedContent,
		"an Anthropic stream signature must not be relabeled as OpenAI Responses encrypted_content")
}

func requireString(t *testing.T, value *string) string {
	t.Helper()
	require.NotNil(t, value)
	return *value
}

func itemTypes(items []Item) []string {
	types := make([]string, len(items))
	for index, item := range items {
		types[index] = item.Type
	}
	return types
}

func findItemByType(t *testing.T, items []Item, itemType string) Item {
	t.Helper()
	for _, item := range items {
		if item.Type == itemType {
			return item
		}
	}
	t.Fatalf("missing item type %q", itemType)
	return Item{}
}
