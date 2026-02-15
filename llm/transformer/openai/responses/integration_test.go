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
		name        string
		requestFile string
	}{
		{
			name:        "simple request array",
			requestFile: `simple.request.json`,
		},
		{
			name:        "single array",
			requestFile: `single_array.request.json`,
		},
		{
			name:        "reasoning request",
			requestFile: `reasoning.request.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wantReq Request

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

			var gotReq Request

			err = json.Unmarshal(outboundReq.Body, &gotReq)
			require.NoError(t, err)

			if !xtest.Equal(wantReq, gotReq, cmpopts.IgnoreFields(Item{}, "EncryptedContent")) {
				t.Errorf("wantReq != gotReq\n%s", cmp.Diff(wantReq, gotReq))
			}
		})
	}
}
