package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// ModerationCreateRequest is the OpenAI /v1/moderations request body.
type ModerationCreateRequest struct {
	Input llm.ModerationInput `json:"input"`
	Model string              `json:"model,omitempty"`
}

// ModerationCreateResponse is the OpenAI /v1/moderations response body.
type ModerationCreateResponse struct {
	ID      string                        `json:"id"`
	Model   string                        `json:"model"`
	Results []llm.ModerationClassification `json:"results"`
}

// transformModerationRequest transforms unified llm.Request to HTTP moderation request.
func (t *OutboundTransformer) transformModerationRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	if llmReq.Moderation == nil {
		return nil, fmt.Errorf("moderation request is nil in llm.Request")
	}

	modReq := ModerationCreateRequest{
		Input: llmReq.Moderation.Input,
		Model: llmReq.Model,
	}

	body, err := json.Marshal(modReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal moderation request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	url := t.buildModerationURL()
	apiKey := t.config.APIKeyProvider.Get(ctx)

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: apiKey,
		},
		RequestType: string(llm.RequestTypeModeration),
		APIFormat:   string(llm.APIFormatOpenAIModeration),
	}, nil
}

func (t *OutboundTransformer) buildModerationURL() string {
	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath
	}

	return t.config.BaseURL + "/moderations"
}

// transformModerationResponse transforms HTTP moderation response to unified llm.Response.
func (t *OutboundTransformer) transformModerationResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var modResp ModerationCreateResponse
	if err := json.Unmarshal(httpResp.Body, &modResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal moderation response: %w", err)
	}

	return &llm.Response{
		ID:          modResp.ID,
		Model:       modResp.Model,
		RequestType: llm.RequestTypeModeration,
		APIFormat:   llm.APIFormatOpenAIModeration,
		Moderation: &llm.ModerationResponse{
			Results: modResp.Results,
		},
	}, nil
}
