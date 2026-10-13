package responses_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

const protocolRequest = `{"model":"synthetic","previous_response_id":"resp_provider","input":[{"type":"message","id":"msg_provider","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,c3ludGhldGlj","detail":"original","future_image":{"canary":1}}]},{"type":"function_call","id":"fc_provider","call_id":"call_provider","name":"synthetic","arguments":"{}","future_call":true},{"type":"function_call_output","call_id":"call_provider","output":"tool-canary","future_result":2},{"type":"reasoning","id":"rs_provider","encrypted_content":"synthetic-encrypted","summary":[],"future_reasoning":true},{"type":"image_generation_call","id":"ig_provider","action":"edit","result":"synthetic-image","future_action":true}],"tools":[{"type":"function","name":"synthetic","parameters":{"type":"object"},"future_tool":true}],"additional_tools":[{"type":"custom","name":"extra","format":{"type":"text"}}],"future_request":{"canary":"raw"}}`

const protocolResponse = `{"id":"resp_provider","object":"response","model":"synthetic","status":"completed","output":[{"type":"message","id":"msg_provider","role":"assistant","content":[{"type":"output_text","text":"canary","annotations":[],"future_content":true}]},{"type":"function_call","id":"fc_provider","call_id":"call_provider","name":"synthetic","arguments":"{}"},{"type":"image_generation_call","id":"ig_provider","action":"edit","result":"synthetic-image","future_image":true}],"future_response":{"canary":42}}`

func TestResponsesTransportConversionPreservesItems(t *testing.T) {
	for _, source := range []llm.APIFormat{llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseWebSocket} {
		for _, target := range []llm.APIFormat{llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseWebSocket} {
			t.Run(source.String()+" to "+target.String(), func(t *testing.T) {
				// Given: identical provider content under either transport envelope.
				body := []byte(protocolRequest)
				if source == llm.APIFormatOpenAIResponseWebSocket {
					body = []byte(`{"type":"response.create","stream_id":"lane","generate":true,` + protocolRequest[1:])
				}
				original := string(body)
				// When: the shared production codec changes only the transport.
				converted, err := responses.AdaptRequestBody(body, source, target)
				// Then: all raw items, IDs, tools and unknown fields survive.
				require.NoError(t, err)
				expected := protocolRequest
				if target == llm.APIFormatOpenAIResponseWebSocket {
					expected = `{"type":"response.create",` + protocolRequest[1:]
				}
				require.JSONEq(t, expected, string(converted))
				require.Equal(t, original, string(body))
				responseBody, err := responses.AdaptResponseBody([]byte(protocolResponse), source, target)
				require.NoError(t, err)
				require.JSONEq(t, protocolResponse, string(responseBody))
				outbound, err := responses.NewOutboundTransformer("https://example.invalid", "synthetic")
				require.NoError(t, err)
				parsed, err := outbound.TransformResponse(context.Background(), &httpclient.Response{StatusCode: 200, Body: responseBody})
				require.NoError(t, err)
				require.Equal(t, "resp_provider", parsed.ID)
			})
		}
	}
}

func TestResponsesTransportConversionPreservesItemsEvents(t *testing.T) {
	for _, source := range []llm.APIFormat{llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseWebSocket} {
		for _, target := range []llm.APIFormat{llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseWebSocket} {
			for _, data := range []string{
				`{"type":"response.output_item.done","output_index":0,"item":{"type":"image_generation_call","id":"ig_provider","action":"edit","result":"synthetic-image","future_item":true},"future_event":true}`,
				fmt.Sprintf(`{"type":"response.completed","response":%s,"future_event":true}`, protocolResponse),
			} {
				// Given
				event := &httpclient.StreamEvent{Data: []byte(data), LastEventID: "provider-event"}
				// When
				converted, err := responses.AdaptStreamEvent(event, source, target)
				// Then
				require.NoError(t, err)
				require.JSONEq(t, data, string(converted.Data))
				require.Equal(t, "provider-event", converted.LastEventID)
				require.NotEmpty(t, converted.Type)
			}
		}
	}
}

func TestResponsesTransportConversionPreservesItemsRejectsMalformed(t *testing.T) {
	for _, body := range []string{`{`, `null`, `[]`, `{}`, `{"model":2}`, `{"model":"synthetic","type":"response.cancel"}`} {
		t.Run(body, func(t *testing.T) {
			// Given / When
			converted, err := responses.AdaptRequestBody([]byte(body), llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponse)
			// Then
			require.Error(t, err)
			require.Nil(t, converted)
		})
	}
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIResponseCompact} {
		// Given / When
		converted, err := responses.AdaptRequestBody([]byte(protocolRequest), format, llm.APIFormatOpenAIResponseWebSocket)
		// Then
		require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
		require.Nil(t, converted)
	}
}

func TestResponsesTransportConversionPreservesItemsRejectsMisleadingSuccess(t *testing.T) {
	for _, body := range []string{`{`, `null`, `[]`, `{"status":"completed"}`, `{"id":2,"status":"completed"}`, `{"id":"resp_provider","status":"completed","output":"invalid"}`} {
		t.Run(body, func(t *testing.T) {
			// Given / When
			converted, err := responses.AdaptResponseBody([]byte(body), llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponse)
			// Then
			require.ErrorIs(t, err, transformer.ErrInvalidResponse)
			require.Nil(t, converted)
		})
	}
	// Given / When: a completed label without a provider response is insufficient.
	event, err := responses.AdaptStreamEvent(&httpclient.StreamEvent{Data: []byte(`{"type":"response.completed"}`)}, llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponse)
	// Then
	require.ErrorIs(t, err, transformer.ErrInvalidResponse)
	require.Nil(t, event)
	// Given / When: the terminal label disagrees with the provider status.
	event, err = responses.AdaptStreamEvent(&httpclient.StreamEvent{Data: []byte(`{"type":"response.completed","response":{"id":"resp_provider","status":"failed"}}`)}, llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponse)
	// Then
	require.ErrorIs(t, err, transformer.ErrInvalidResponse)
	require.Nil(t, event)
}

func TestResponsesTransportConversionPreservesItemsStateIsolation(t *testing.T) {
	// Given: borrowed input bytes and a stale SSE label.
	body := []byte(protocolResponse)
	event := &httpclient.StreamEvent{Type: "response.completed", Data: []byte(`{"type":"response.failed","response":{"id":"resp_provider","status":"failed"}}`)}
	// When
	converted, err := responses.AdaptResponseBody(body, llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseWebSocket)
	// Then: returned payload ownership is independent of the source.
	require.NoError(t, err)
	body[0] = '!'
	require.JSONEq(t, protocolResponse, string(converted))
	_, err = responses.AdaptStreamEvent(event, llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseWebSocket)
	require.ErrorIs(t, err, transformer.ErrInvalidResponse)
}

func TestResponsesTransportConversionPreservesItemsWarmup(t *testing.T) {
	// Given
	body := []byte(`{"type":"response.create","model":"synthetic","generate":false,"input":"canary"}`)
	// When
	converted, err := responses.AdaptRequestBody(body, llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponse)
	// Then: a warmup cannot silently become an HTTP generation.
	require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
	require.Nil(t, converted)
	converted, err = responses.AdaptRequestBody(body, llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponseWebSocket)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(converted))
}
