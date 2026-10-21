package cline

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/spf13/cast"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func parseClineError(statusCode int, body []byte) *llm.ResponseError {
	var payload struct {
		Success *bool `json:"success"`
		Error   any   `json:"error"`
		Errors  any   `json:"errors"`
	}

	if len(body) > 0 && json.Unmarshal(body, &payload) == nil {
		if payload.Error != nil {
			return clineResponseError(statusCode, payload.Error)
		}
		if payload.Errors != nil {
			return clineResponseError(statusCode, payload.Errors)
		}
	}

	message := http.StatusText(statusCode)
	if message == "" {
		message = http.StatusText(http.StatusBadGateway)
	}

	return clineResponseError(statusCode, message)
}

func parseClineErrorEvent(event *httpclient.StreamEvent) *llm.ResponseError {
	if event == nil {
		return nil
	}

	if event.Type == "error" && len(event.Data) == 0 {
		return clineResponseError(http.StatusBadGateway, "stream error")
	}

	if len(event.Data) == 0 {
		return nil
	}

	var payload struct {
		Success *bool  `json:"success"`
		Error   any    `json:"error"`
		Errors  any    `json:"errors"`
		Event   string `json:"event"`
		Data    struct {
			Error any `json:"error"`
		} `json:"data"`
	}
	if json.Unmarshal(event.Data, &payload) != nil {
		return nil
	}

	if payload.Success != nil && !*payload.Success {
		if payload.Error != nil {
			return clineResponseError(http.StatusBadGateway, payload.Error)
		}
		if payload.Errors != nil {
			return clineResponseError(http.StatusBadGateway, payload.Errors)
		}

		return clineResponseError(http.StatusBadGateway, "stream error")
	}
	if payload.Error != nil {
		return clineResponseError(http.StatusBadGateway, payload.Error)
	}
	if payload.Errors != nil {
		return clineResponseError(http.StatusBadGateway, payload.Errors)
	}
	if event.Type == "error" || payload.Event == "error" {
		if payload.Data.Error != nil {
			return clineResponseError(http.StatusBadGateway, payload.Data.Error)
		}

		return clineResponseError(http.StatusBadGateway, "stream error")
	}

	return nil
}

func clineErrorMessage(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case map[string]any:
		if msg := cast.ToString(v["message"]); msg != "" {
			return msg
		}
		return cast.ToString(v)
	default:
		return cast.ToString(v)
	}
}

// isClineBalanceExhausted reports whether an error message indicates the
// Cline credits balance is exhausted (e.g. "Insufficient balance. Your Cline
// Credits balance is $-0.00"). Such failures are permanent until topped up.
func isClineBalanceExhausted(message string) bool {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "insufficient balance") {
		return true
	}
	if strings.Contains(lower, "balance") &&
		(strings.Contains(lower, "cline") || strings.Contains(lower, "credit")) {
		return true
	}
	return false
}

func clineResponseError(statusCode int, raw any) *llm.ResponseError {
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	detail := llm.ErrorDetail{Type: "api_error"}

	switch v := raw.(type) {
	case string:
		detail.Message = v
	case map[string]any:
		detail.Message = cast.ToString(v["message"])
		detail.Type = cast.ToString(v["type"])
		detail.Code = cast.ToString(v["code"])
		detail.Param = cast.ToString(v["param"])
		detail.RequestID = cast.ToString(v["request_id"])
	case []any:
		messages := make([]string, 0, len(v))
		for _, item := range v {
			if msg := clineErrorMessage(item); msg != "" {
				messages = append(messages, msg)
			}
		}
		detail.Message = strings.Join(messages, "; ")
	default:
		detail.Message = cast.ToString(v)
	}

	if detail.Type == "" {
		detail.Type = "api_error"
	}
	if detail.Message == "" {
		detail.Message = http.StatusText(statusCode)
	}
	if detail.Message == "" {
		detail.Message = "Cline API error"
	}

	// Balance/credit exhaustion is permanent: map it to 402 Payment Required
	// so it fails fast instead of entering the 5xx retry/failover loop.
	if statusCode != http.StatusPaymentRequired && isClineBalanceExhausted(detail.Message) {
		statusCode = http.StatusPaymentRequired
	}

	return &llm.ResponseError{
		StatusCode: statusCode,
		Detail:     detail,
	}
}
