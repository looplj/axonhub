package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/looplj/axonhub/axon/agent"
)

func wrapAPIError(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		msg := extractErrorMessage(apiErr)
		if msg == "" {
			msg = "anthropic: request failed"
		} else {
			msg = fmt.Sprintf("anthropic: %s", msg)
		}

		return &agent.ProviderError{
			StatusCode: apiErr.StatusCode,
			Message:    msg,
		}
	}
	return fmt.Errorf("anthropic: %w", err)
}

func extractErrorMessage(apiErr *anthropic.Error) string {
	raw := apiErr.RawJSON()
	if raw == "" {
		return ""
	}

	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &body) == nil && body.Error.Message != "" {
		return body.Error.Message
	}
	return ""
}
