package responses

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
)

func TestTransformRequest_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name         string
		requestFile  string
		expectedFile string
	}{
		{
			name:         "simple request array",
			requestFile:  `simple.request.json`,
			expectedFile: `simple.request.json`,
		},
		{
			name:         "single array",
			requestFile:  `single_array.request.json`,
			expectedFile: `single_array.request.json`,
		},
		{
			name:         "reasoning request",
			requestFile:  `reasoning.request.json`,
			expectedFile: `reasoning.request.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputReq Request

			err := xtest.LoadTestData(t, tt.requestFile, &inputReq)
			require.NoError(t, err)

			var expectedReq Request

			err = xtest.LoadTestData(t, tt.expectedFile, &expectedReq)
			require.NoError(t, err)

			var buf bytes.Buffer

			decoder := json.NewEncoder(&buf)
			decoder.SetEscapeHTML(false)

			if err := decoder.Encode(inputReq); err != nil {
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

			var gotReq Request

			err = json.Unmarshal(outboundReq.Body, &gotReq)
			require.NoError(t, err)

			if !xtest.Equal(expectedReq, gotReq, cmpopts.IgnoreFields(Item{}, "EncryptedContent")) {
				t.Errorf("wantReq != gotReq\n%s", cmp.Diff(expectedReq, gotReq))
			}
		})
	}
}

func TestTransformRequest_PreservesOfficialRawTopLevelFields(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	tests := []struct {
		name string
		body string
		keys []string
	}{
		{
			name: "prompt can provide the request content",
			body: `{"model":"gpt-5.6","prompt":{"id":"pmpt_123","version":"2","variables":{"topic":"frogs"}}}`,
			keys: []string{"prompt"},
		},
		{
			name: "conversation can provide the request context",
			body: `{"model":"gpt-5.6","conversation":{"id":"conv_123"}}`,
			keys: []string{"conversation"},
		},
		{
			name: "current optional request fields",
			body: `{"model":"gpt-5.6","input":"hello","context_management":[{"type":"compaction","compact_threshold":12000}],"moderation":{"model":"omni-moderation-latest","policy":{"input":{"mode":"score"},"output":{"mode":"score"}}},"prompt_cache_options":{"mode":"implicit","ttl":"30m"}}`,
			keys: []string{"context_management", "moderation", "prompt_cache_options"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inboundRequest := &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    []byte(tt.body),
			}
			llmRequest, err := inboundTransformer.TransformRequest(t.Context(), inboundRequest)
			require.NoError(t, err)

			outboundRequest, err := outboundTransformer.TransformRequest(t.Context(), llmRequest)
			require.NoError(t, err)

			var want map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tt.body), &want))
			var got map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(outboundRequest.Body, &got))
			for _, key := range tt.keys {
				require.JSONEq(t, string(want[key]), string(got[key]), key)
			}
			if _, hasInput := want["input"]; !hasInput {
				_, gotHasInput := got["input"]
				require.False(t, gotHasInput)
			}
		})
	}
}
