package openai

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

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
