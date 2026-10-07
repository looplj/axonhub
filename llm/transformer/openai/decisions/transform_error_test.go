package decisions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestInboundTransformer_TransformErrorPreservesResponseError(t *testing.T) {
	transformerInstance := NewInboundTransformer()

	for _, test := range []struct {
		name   string
		status int
	}{
		{name: "rate limited", status: http.StatusTooManyRequests},
		{name: "provider failure", status: http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			detail := llm.ErrorDetail{Code: "provider_error", Message: "provider failed", Type: "api_error"}
			result := transformerInstance.TransformError(context.Background(), &llm.ResponseError{StatusCode: test.status, Detail: detail})

			require.Equal(t, test.status, result.StatusCode)
			require.Equal(t, http.StatusText(test.status), result.Status)
			var body struct {
				Detail llm.ErrorDetail `json:"error"`
			}
			require.NoError(t, json.Unmarshal(result.Body, &body))
			require.Equal(t, detail, body.Detail)
		})
	}
}

func TestInboundTransformer_TransformErrorPassesThroughHTTPError(t *testing.T) {
	transformerInstance := NewInboundTransformer()

	for _, test := range []struct {
		name   string
		status int
	}{
		{name: "rate limited", status: http.StatusTooManyRequests},
		{name: "provider failure", status: http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			httpErr := &httpclient.Error{StatusCode: test.status, Status: http.StatusText(test.status), Body: []byte(`{"error":{"message":"provider failed"}}`)}

			require.Same(t, httpErr, transformerInstance.TransformError(context.Background(), httpErr))
		})
	}
}

func TestInboundTransformer_TransformErrorMapsInvalidRequest(t *testing.T) {
	transformerInstance := NewInboundTransformer()
	result := transformerInstance.TransformError(context.Background(), fmt.Errorf("%w: malformed input", transformer.ErrInvalidRequest))

	require.Equal(t, http.StatusBadRequest, result.StatusCode)
	require.Equal(t, http.StatusText(http.StatusBadRequest), result.Status)
	var body struct {
		Detail llm.ErrorDetail `json:"error"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &body))
	require.Equal(t, "invalid_request_error", body.Detail.Type)
	require.Contains(t, body.Detail.Message, "malformed input")
}

func TestInboundTransformer_TransformErrorMapsUnknownErrorToInternalError(t *testing.T) {
	transformerInstance := NewInboundTransformer()
	result := transformerInstance.TransformError(context.Background(), errors.New("database unavailable"))

	require.Equal(t, http.StatusInternalServerError, result.StatusCode)
	require.Equal(t, http.StatusText(http.StatusInternalServerError), result.Status)
	var body struct {
		Detail llm.ErrorDetail `json:"error"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &body))
	require.Equal(t, llm.ErrorDetail{Message: "database unavailable", Type: "internal_server_error"}, body.Detail)
}
