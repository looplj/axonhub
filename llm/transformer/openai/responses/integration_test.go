package responses

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/samber/lo"
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

			if tt.name == "reasoning request" {
				expectedReq.Input.Items = lo.Filter(expectedReq.Input.Items, func(item Item, _ int) bool {
					return item.Type != "reasoning"
				})
			}

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
