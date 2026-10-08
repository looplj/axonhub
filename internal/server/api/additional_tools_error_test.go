package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestTransformOrchestratorError_MapsUnsupportedAdditionalToolsToInvalidRequest(t *testing.T) {
	for _, inbound := range []struct {
		name  string
		value transformer.Inbound
	}{
		{name: "responses", value: responses.NewInboundTransformer()},
		{name: "compact", value: responses.NewCompactInboundTransformer()},
	} {
		t.Run(inbound.name, func(t *testing.T) {
			err := fmt.Errorf(
				"conversion failed: %w",
				errors.Join(
					transformer.ErrUnsupportedConversion,
					transformer.ErrInvalidRequest,
					errors.New("additional_tools input items cannot be converted to anthropic/messages"),
				),
			)

			httpErr := transformOrchestratorError(t.Context(), err, &orchestrator.ChatCompletionOrchestrator{Inbound: inbound.value})

			require.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
			require.Equal(t, "invalid_request_error", gjson.GetBytes(httpErr.Body, "error.type").String())
			require.Contains(t, gjson.GetBytes(httpErr.Body, "error.message").String(), "additional_tools")
		})
	}
}
