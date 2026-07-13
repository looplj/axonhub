package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	openaiTransformer "github.com/looplj/axonhub/llm/transformer/openai"
)

// R1: non-stream same-family public seam must preserve response status values that
// are not mapped through Chat-like finish reasons today (queued / in_progress).
func TestR1_NonStreamStatus_QueuedAndInProgress_RoundTrip(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	inbound := NewInboundTransformer()

	for _, status := range []string{"queued", "in_progress"} {
		t.Run(status, func(t *testing.T) {
			body := []byte(`{
				"id":"resp_` + status + `",
				"object":"response",
				"created_at":1700000000,
				"model":"gpt-5",
				"status":"` + status + `",
				"output":[]
			}`)

			llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			})
			require.NoError(t, err)
			require.Len(t, llmResp.Choices, 1)
			require.Nil(t, llmResp.Choices[0].FinishReason,
				"Responses-native non-terminal status must not occupy the shared Chat finish_reason carrier")
			require.NotNil(t, llmResp.ProviderExtensions)
			require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses)
			require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses.Response)
			require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses.Response.Status)
			require.Equal(t, status, *llmResp.ProviderExtensions.OpenAIResponses.Response.Status)

			httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
			require.NoError(t, err)

			root := gjson.ParseBytes(httpResp.Body)
			require.Equal(t, status, root.Get("status").String(),
				"same-family replay must preserve status %q (not default to completed)", status)
			require.Equal(t, "resp_"+status, root.Get("id").String())
			require.Equal(t, "gpt-5", root.Get("model").String())
		})
	}
}

// R1: Responses-native queued/in_progress status must not leak through the
// shared Choice.FinishReason carrier when the canonical response is emitted as
// OpenAI Chat.
func TestR1_NonTerminalResponsesStatus_DoesNotLeakToOpenAIChat(t *testing.T) {
	for _, status := range []string{"queued", "in_progress"} {
		t.Run(status, func(t *testing.T) {
			source := []byte(fmt.Sprintf(`{
				"id":"resp_%s_to_chat",
				"object":"response",
				"created_at":1700000006,
				"model":"gpt-5",
				"status":"%s",
				"output":[]
			}`, status, status))

			responsesOutbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			llmResp, err := responsesOutbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: http.StatusOK, Body: source})
			require.NoError(t, err)

			chatInbound := openaiTransformer.NewInboundTransformer()
			chatResp, err := chatInbound.TransformResponse(t.Context(), llmResp)
			require.NoError(t, err)

			var got map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(chatResp.Body, &got))
			var choices []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(got["choices"], &choices))
			require.Len(t, choices, 1)
			require.NotEqual(t, fmt.Sprintf(`"%s"`, status), string(choices[0]["finish_reason"]))
		})
	}
}

// R1: a Responses-native non-terminal status takes precedence over a shared
// terminal finish reason if a mixed canonical response is ever assembled by an
// intermediary. The native sidecar is the same-protocol source of truth.
func TestR1_NativeNonTerminalStatusPrecedesSharedFinishReason(t *testing.T) {
	status := "queued"
	finishReason := "error"
	llmResp := &llm.Response{
		ID:    "resp_native_status_wins",
		Model: "gpt-5",
		Choices: []llm.Choice{{
			Index:        0,
			FinishReason: &finishReason,
			Message:      &llm.Message{Role: "assistant"},
		}},
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Response: &llm.OpenAIResponsesResponseExtensions{Status: &status},
			},
		},
	}

	httpResp, err := NewInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)
	var got Response
	require.NoError(t, json.Unmarshal(httpResp.Body, &got))
	require.NotNil(t, got.Status)
	require.Equal(t, "queued", *got.Status)
}

// R1: a non-terminal Responses envelope with output:[] must retain that empty
// output array instead of receiving a synthetic completed assistant message.
func TestR1_NonTerminalEmptyOutput_DoesNotInventMessage(t *testing.T) {
	for _, status := range []string{"queued", "in_progress", "incomplete", "failed", "canceled"} {
		t.Run(status, func(t *testing.T) {
			source := []byte(fmt.Sprintf(`{
				"id":"resp_%s_empty",
				"object":"response",
				"created_at":1700000005,
				"model":"gpt-5",
				"status":"%s",
				"output":[]
			}`, status, status))

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{StatusCode: http.StatusOK, Body: source})
			require.NoError(t, err)

			httpResp, err := NewInboundTransformer().TransformResponse(context.Background(), llmResp)
			require.NoError(t, err)

			var got Response
			require.NoError(t, json.Unmarshal(httpResp.Body, &got))
			require.NotNil(t, got.Status)
			require.Equal(t, status, *got.Status)
			require.Empty(t, got.Output)
		})
	}
}

// R1: non-stream incomplete_details.reason must survive Responses JSON → llm → Responses JSON.
func TestR1_NonStreamIncompleteDetails_RoundTrip(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	inbound := NewInboundTransformer()

	body := []byte(`{
		"id":"resp_incomplete_details",
		"object":"response",
		"created_at":1700000001,
		"model":"gpt-5",
		"status":"incomplete",
		"incomplete_details":{"reason":"max_output_tokens"},
		"output":[{
			"id":"msg_incomplete_1",
			"type":"message",
			"role":"assistant",
			"status":"incomplete",
			"content":[{"type":"output_text","text":"partial answer","annotations":[]}]
		}]
	}`)

	llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].FinishReason)
	require.Equal(t, "length", *llmResp.Choices[0].FinishReason)
	require.NotNil(t, llmResp.ProviderExtensions)
	require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses.Response)
	require.JSONEq(t, `{"reason":"max_output_tokens"}`,
		string(llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields["incomplete_details"]),
		"incomplete_details must ride the Responses-native response sidecar")

	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	root := gjson.ParseBytes(httpResp.Body)
	require.Equal(t, "incomplete", root.Get("status").String())
	require.Equal(t, "max_output_tokens", root.Get("incomplete_details.reason").String())
	require.Equal(t, "partial answer", root.Get("output.0.content.0.text").String())
}

// R1: explicit incomplete_details:null is not the same as field absence.
// Public Responses HTTP -> llm -> Responses HTTP must preserve both states.
func TestR1_NonStreamIncompleteDetails_NullPresence_RoundTrip(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	inbound := NewInboundTransformer()

	for _, tc := range []struct {
		name           string
		body           string
		wantExplicitNull bool
	}{
		{
			name: "explicit_null",
			body: `{
				"id":"resp_incomplete_details_null",
				"object":"response",
				"created_at":1700000003,
				"model":"gpt-5",
				"status":"completed",
				"incomplete_details":null,
				"output":[{
					"id":"msg_incomplete_null",
					"type":"message",
					"role":"assistant",
					"status":"completed",
					"content":[{"type":"output_text","text":"done","annotations":[]}]
				}]
			}`,
			wantExplicitNull: true,
		},
		{
			name: "absent",
			body: `{
				"id":"resp_incomplete_details_absent",
				"object":"response",
				"created_at":1700000004,
				"model":"gpt-5",
				"status":"completed",
				"output":[{
					"id":"msg_incomplete_absent",
					"type":"message",
					"role":"assistant",
					"status":"completed",
					"content":[{"type":"output_text","text":"done","annotations":[]}]
				}]
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(tc.body),
			})
			require.NoError(t, err)

			httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
			require.NoError(t, err)
			root := gjson.ParseBytes(httpResp.Body)

			if tc.wantExplicitNull {
				require.NotNil(t, llmResp.ProviderExtensions)
				require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses)
				require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses.Response)
				raw, ok := llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields["incomplete_details"]
				require.True(t, ok, "explicit null must be captured into response RawTopLevelFields")
				require.JSONEq(t, `null`, string(raw))
				require.True(t, root.Get("incomplete_details").Exists(),
					"explicit incomplete_details:null must survive same-family replay")
				require.Equal(t, gjson.Null, root.Get("incomplete_details").Type,
					"replay must emit JSON null, not an object or omission")
				return
			}

			if llmResp.ProviderExtensions != nil && llmResp.ProviderExtensions.OpenAIResponses != nil &&
				llmResp.ProviderExtensions.OpenAIResponses.Response != nil {
				require.NotContains(t, llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields, "incomplete_details",
					"absent incomplete_details must not invent a sidecar entry")
			}
			require.False(t, root.Get("incomplete_details").Exists(),
				"absent incomplete_details must remain omitted on replay")
		})
	}
}

// R1: message output content part type=refusal must round-trip on the non-stream public seam.
func TestR1_NonStreamRefusalContent_RoundTrip(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	inbound := NewInboundTransformer()

	body := []byte(`{
		"id":"resp_refusal",
		"object":"response",
		"created_at":1700000002,
		"model":"gpt-5",
		"status":"completed",
		"output":[{
			"id":"msg_refusal_1",
			"type":"message",
			"role":"assistant",
			"status":"completed",
			"content":[{
				"type":"refusal",
				"refusal":"I cannot help with that request."
			}]
		}]
	}`)

	llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)
	require.Len(t, llmResp.Choices, 1)
	require.NotNil(t, llmResp.Choices[0].Message)
	require.Equal(t, "I cannot help with that request.", llmResp.Choices[0].Message.Refusal,
		"refusal content must not disappear into empty assistant content")
	require.Equal(t, "msg_refusal_1", llmResp.Choices[0].Message.ID)

	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	root := gjson.ParseBytes(httpResp.Body)
	require.Equal(t, "completed", root.Get("status").String())
	require.Equal(t, "message", root.Get("output.0.type").String())
	require.Equal(t, "msg_refusal_1", root.Get("output.0.id").String())
	require.Equal(t, "refusal", root.Get("output.0.content.0.type").String())
	require.Equal(t, "I cannot help with that request.", root.Get("output.0.content.0.refusal").String())
	require.False(t, root.Get("output.0.content.0.text").Exists(),
		"refusal part must not be rewritten as output_text")
}

// Guard: convertToResponsesAPIResponse must not invent incomplete_details when absent.
func TestR1_ConvertToResponsesAPIResponse_NoInventedIncompleteDetails(t *testing.T) {
	resp := convertToResponsesAPIResponse(&llm.Response{
		ID:    "resp_plain",
		Model: "gpt-5",
		Choices: []llm.Choice{{
			FinishReason: lo.ToPtr("stop"),
			Message: &llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("ok"),
				},
			},
		}},
	})
	require.Nil(t, resp.IncompleteDetails)
	require.Equal(t, "completed", *resp.Status)
}

// Guard: empty refusal must not invent a refusal content part.
func TestR1_ConvertToResponsesAPIResponse_EmptyRefusalOmitted(t *testing.T) {
	resp := convertToResponsesAPIResponse(&llm.Response{
		ID:    "resp_no_refusal",
		Model: "gpt-5",
		Choices: []llm.Choice{{
			FinishReason: lo.ToPtr("stop"),
			Message: &llm.Message{
				Role:    "assistant",
				Refusal: "",
				Content: llm.MessageContent{
					Content: lo.ToPtr("hello"),
				},
			},
		}},
	})
	require.Len(t, resp.Output, 1)
	require.NotNil(t, resp.Output[0].Content)
	for _, item := range resp.Output[0].Content.Items {
		require.NotEqual(t, "refusal", item.Type)
	}
}

func TestR1_ItemRefusalJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"type":"refusal","refusal":"blocked by policy"}`)
	var item Item
	require.NoError(t, json.Unmarshal(raw, &item))
	require.Equal(t, "refusal", item.Type)
	require.NotNil(t, item.Refusal)
	require.Equal(t, "blocked by policy", *item.Refusal)

	out, err := json.Marshal(item)
	require.NoError(t, err)
	require.JSONEq(t, string(raw), string(out))
}

// R1 follow-up: completed_at and output_text are Responses-native response
// envelope fields. They must survive the public Responses HTTP -> llm ->
// Responses HTTP seam through the response sidecar, while source omission stays
// omission.
func TestR1_NonStreamCompletedEnvelopeFields_RoundTrip(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	inbound := NewInboundTransformer()

	for _, tc := range []struct {
		name          string
		body          string
		wantCompleted bool
	}{
		{
			name: "provided",
			body: `{
				"id":"resp_completed_envelope",
				"object":"response",
				"created_at":1700000010,
				"completed_at":1700000012,
				"model":"gpt-5",
				"status":"completed",
				"output_text":"final answer",
				"output":[{
					"id":"msg_completed_envelope",
					"type":"message",
					"role":"assistant",
					"status":"completed",
					"content":[{"type":"output_text","text":"final answer","annotations":[]}]
				}]
			}`,
			wantCompleted: true,
		},
		{
			name: "omitted",
			body: `{
				"id":"resp_completed_envelope_omitted",
				"object":"response",
				"created_at":1700000010,
				"model":"gpt-5",
				"status":"completed",
				"output":[{
					"id":"msg_completed_envelope_omitted",
					"type":"message",
					"role":"assistant",
					"status":"completed",
					"content":[{"type":"output_text","text":"final answer","annotations":[]}]
				}]
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(tc.body),
			})
			require.NoError(t, err)

			httpResp, err := inbound.TransformResponse(t.Context(), llmResp)
			require.NoError(t, err)
			root := gjson.ParseBytes(httpResp.Body)

			if !tc.wantCompleted {
				require.False(t, root.Get("completed_at").Exists(), "omitted completed_at must not be invented")
				require.False(t, root.Get("output_text").Exists(), "omitted output_text must not be invented")
				if llmResp.ProviderExtensions != nil && llmResp.ProviderExtensions.OpenAIResponses != nil &&
					llmResp.ProviderExtensions.OpenAIResponses.Response != nil {
					require.NotContains(t, llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields, "completed_at")
					require.NotContains(t, llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields, "output_text")
				}
				return
			}

			require.NotNil(t, llmResp.ProviderExtensions)
			require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses)
			require.NotNil(t, llmResp.ProviderExtensions.OpenAIResponses.Response)
			require.JSONEq(t, `1700000012`, string(llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields["completed_at"]))
			require.JSONEq(t, `"final answer"`, string(llmResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields["output_text"]))
			require.NotContains(t, llmResp.TransformerMetadata, "completed_at")
			require.NotContains(t, llmResp.TransformerMetadata, "output_text")
			require.Equal(t, int64(1700000012), root.Get("completed_at").Int())
			require.Equal(t, "final answer", root.Get("output_text").String())
		})
	}
}

func TestR1_RestoreResponseTopLevelFields_TypedEnvelopeWins(t *testing.T) {
	llmResp := &llm.Response{}
	ext := llm.EnsureOpenAIResponsesResponseExtensions(llmResp)
	ext.RawTopLevelFields = map[string]json.RawMessage{
		"completed_at": json.RawMessage(`1700000099`),
		"output_text":  json.RawMessage(`"sidecar text"`),
	}

	body, err := restoreOpenAIResponsesResponseTopLevelFields(
		[]byte(`{"completed_at":1700000012,"output_text":"typed text"}`),
		llmResp,
	)
	require.NoError(t, err)
	root := gjson.ParseBytes(body)
	require.Equal(t, int64(1700000012), root.Get("completed_at").Int())
	require.Equal(t, "typed text", root.Get("output_text").String())
}
