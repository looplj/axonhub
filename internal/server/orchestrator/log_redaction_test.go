package orchestrator

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRedactedRequestLogSummary_DoesNotExposeBodyOrHeaderValues(t *testing.T) {
	req := &httpclient.Request{
		Method:    http.MethodPost,
		Path:      "/v1/responses",
		APIFormat: "openai/responses",
		Headers: http.Header{
			"Authorization": []string{"Bearer secret-token"},
			"X-Trace":       []string{"trace-123"},
		},
		Body: []byte(`{"input":"secret prompt"}`),
	}

	summary := redactedRequestLogSummary(req)
	rendered := fmt.Sprintf("%+v", summary)

	require.Equal(t, len(req.Body), summary.BodyBytes)
	require.Contains(t, summary.HeaderNames, "Authorization")
	require.NotContains(t, rendered, "secret prompt")
	require.NotContains(t, rendered, "secret-token")
	require.NotContains(t, rendered, "trace-123")
}
