package openai

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	responses "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestOpenAIChatRequestNRawRoundTrip(t *testing.T) {
	for _, fixture := range []struct {
		name string
		path string
		want string
	}{
		{name: "one", path: "testdata/openai-n-1.request.json", want: "1"},
		{name: "multiple", path: "testdata/openai-n-3.request.json", want: "3"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			body, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			inbound := NewInboundTransformer()
			llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    body,
			})
			require.NoError(t, err)

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
			require.NoError(t, err)

			var outboundBody map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
			require.JSONEq(t, fixture.want, string(outboundBody["n"]))

			requestType := reflect.TypeOf(*llmReq)
			_, hasCommonN := requestType.FieldByName("N")
			require.False(t, hasCommonN, "Chat-native n must not widen llm.Request")
		})
	}
}

func TestOpenAIChatRequestNIsNotSynthesizedForResponses(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-n-3.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	outbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	// Responses has no equivalent multi-choice request control, so omission is
	// the documented unsupported outcome; do not synthesize multiple outputs.
	require.NotContains(t, outboundBody, "n")
}

func TestOpenAIChatRequestPromptCacheRetentionRawRoundTrip(t *testing.T) {
	for _, fixture := range []struct {
		name string
		path string
		want string
	}{
		{name: "known-24h", path: "testdata/openai-prompt-cache-retention-24h.request.json", want: `"24h"`},
		{name: "unknown-future", path: "testdata/openai-prompt-cache-retention-future.request.json", want: `"future-policy-xyz"`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			body, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			inbound := NewInboundTransformer()
			llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    body,
			})
			require.NoError(t, err)

			// Keep storage on the Chat raw request sidecar, not common abstraction.
			requestType := reflect.TypeOf(*llmReq)
			_, hasCommon := requestType.FieldByName("PromptCacheRetention")
			require.False(t, hasCommon, "Chat-native prompt_cache_retention must not widen llm.Request")

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
			require.NoError(t, err)

			var outboundBody map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
			require.JSONEq(t, fixture.want, string(outboundBody["prompt_cache_retention"]))
		})
	}
}

func TestOpenAIChatRequestPromptCacheRetentionBridgesKnownValuesToResponses(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-prompt-cache-retention-24h.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	// S4: Chat PE owns the raw field after inbound attach.
	require.NotNil(t, llm.OpenAIChatRequestExtension(llmReq))
	require.Contains(t, llm.OpenAIChatRequestExtension(llmReq).RawTopLevelFields, "prompt_cache_retention")

	outbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	// Known values (in_memory / 24h) are an explicit tested Chat→Responses bridge.
	require.JSONEq(t, `"24h"`, string(outboundBody["prompt_cache_retention"]))
}

func TestOpenAIChatRequestOutputControlsRawRoundTrip(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		path  string
		field string
	}{
		{name: "audio", path: "testdata/openai-audio.request.json", field: "audio"},
		{name: "prediction-string", path: "testdata/openai-prediction-string.request.json", field: "prediction"},
		{name: "prediction-parts", path: "testdata/openai-prediction-parts.request.json", field: "prediction"},
		{name: "moderation", path: "testdata/openai-moderation.request.json", field: "moderation"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			body, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			inbound := NewInboundTransformer()
			llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    body,
			})
			require.NoError(t, err)

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
			require.NoError(t, err)

			var source, outboundBody map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &source))
			require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
			require.Contains(t, outboundBody, fixture.field)
			require.JSONEq(t, string(source[fixture.field]), string(outboundBody[fixture.field]))

			// Only the requested field should be synthesized from raw preserve.
			for _, other := range []string{"audio", "prediction", "moderation"} {
				if other == fixture.field {
					continue
				}
				if _, ok := source[other]; !ok {
					require.NotContains(t, outboundBody, other)
				}
			}

			requestType := reflect.TypeOf(*llmReq)
			for _, name := range []string{"Audio", "Prediction", "Moderation"} {
				_, has := requestType.FieldByName(name)
				require.False(t, has, "field %s must not widen llm.Request", name)
			}
		})
	}
}

func TestOpenAIChatRequestOutputControlsNotSynthesizedForResponses(t *testing.T) {
	for _, path := range []string{
		"testdata/openai-audio.request.json",
		"testdata/openai-prediction-string.request.json",
		"testdata/openai-prediction-parts.request.json",
		"testdata/openai-moderation.request.json",
	} {
		t.Run(path, func(t *testing.T) {
			body, err := os.ReadFile(path)
			require.NoError(t, err)

			inbound := NewInboundTransformer()
			llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    body,
			})
			require.NoError(t, err)

			outbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
			require.NoError(t, err)

			var outboundBody map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
			for _, field := range []string{"audio", "prediction", "moderation"} {
				require.NotContains(t, outboundBody, field)
			}
		})
	}
}

func TestOpenAIChatRequestWebSearchOptionsRawRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-web-search-options.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.Contains(t, outboundBody, "web_search_options")
	require.JSONEq(t, string(source["web_search_options"]), string(outboundBody["web_search_options"]))
	require.Contains(t, outboundBody, "tools")
	require.JSONEq(t, string(source["tools"]), string(outboundBody["tools"]))

	requestType := reflect.TypeOf(*llmReq)
	_, has := requestType.FieldByName("WebSearchOptions")
	require.False(t, has, "web_search_options must not widen llm.Request")
}

func TestOpenAIChatRequestWebSearchOptionsNotSynthesizedForResponses(t *testing.T) {
	body, err := os.ReadFile("testdata/openai-web-search-options.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	outbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.NotContains(t, outboundBody, "web_search_options")
}
