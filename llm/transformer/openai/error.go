package openai

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/spf13/cast"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TransformOpenAIError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	var openaiError struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Param     string `json:"param,omitempty"`
			Code      any    `json:"code"`
			RequestID string `json:"request_id,omitempty"`
		} `json:"error"`
		Errors struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Param     string `json:"param,omitempty"`
			Code      any    `json:"code"`
			RequestID string `json:"request_id,omitempty"`
		} `json:"errors"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && (openaiError.Error.Message != "" || openaiError.Errors.Message != "") {
		errDetail := openaiError.Error
		if errDetail.Message == "" {
			errDetail = openaiError.Errors
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Message:   errDetail.Message,
				Type:      errDetail.Type,
				Param:     errDetail.Param,
				Code:      cast.ToString(errDetail.Code),
				RequestID: errDetail.RequestID,
			},
		}
	}

	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(rawErr.StatusCode),
			Type:    "api_error",
		},
	}
}
