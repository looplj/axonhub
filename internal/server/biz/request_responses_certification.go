package biz

import (
	"context"
	"encoding/json"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent/request"
)

func (s *RequestService) HasCompletedResponsesOrigin(ctx context.Context, responseID string) (bool, error) {
	return s.entFromContext(ctx).Request.Query().Where(
		request.ExternalIDEQ(responseID),
		request.StatusEQ(request.StatusCompleted),
		request.FormatIn("openai/responses", "openai/responses-ws"),
	).Exist(ctx)
}

func certifiedResponsesSnapshot(requestBody, responseBody []byte, responseID string) bool {
	previous := gjson.GetBytes(requestBody, "previous_response_id")
	if previous.Exists() && previous.String() != "" {
		return false
	}
	response := gjson.ParseBytes(responseBody)
	if response.Get("id").String() != responseID || response.Get("status").String() != "completed" || !response.Get("output").IsArray() {
		return false
	}
	input := gjson.GetBytes(requestBody, "input")
	if input.Type != gjson.String && !input.IsArray() {
		return false
	}
	if input.IsArray() {
		for _, item := range input.Array() {
			if !item.IsObject() {
				return false
			}
		}
	}
	for _, item := range response.Get("output").Array() {
		if !item.IsObject() {
			return false
		}
	}
	return json.Valid(requestBody) && json.Valid(responseBody)
}
