package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
	EndpointPath   string              `json:"endpoint_path,omitempty"`
}

type OutboundTransformer struct {
	config *Config
}

func NewOutboundTransformer(baseURL, apiKey string) (*OutboundTransformer, error) {
	return NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	})
}

func NewOutboundTransformerWithConfig(config *Config) (*OutboundTransformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required")
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.typesafe.ai/v1"
	}
	if config.EndpointPath != "" {
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
	} else {
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
	}

	return &OutboundTransformer{
		config: config,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatTypeSafeSystemOne
}

func (t *OutboundTransformer) AllowPassThroughBody(ctx context.Context, llmReq *llm.Request, providerReq *httpclient.Request) bool {
	return llmReq != nil && llmReq.RequestType == llm.RequestTypeSystemOne
}

func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}
	if llmReq.SystemOne == nil {
		return nil, fmt.Errorf("systemone request is nil")
	}
	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	wireReq := systemOneWireRequest{
		Model:     llmReq.Model,
		State:     llmReq.SystemOne.State,
		Questions: llmReq.SystemOne.Questions,
	}

	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal systemone request: %w", err)
	}

	apiKey := t.config.APIKeyProvider.Get(ctx)

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	url := t.buildURL()

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: apiKey,
		},
		RequestType: string(llm.RequestTypeSystemOne),
		APIFormat:   string(llm.APIFormatTypeSafeSystemOne),
	}, nil
}

func (t *OutboundTransformer) buildURL() string {
	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath
	}
	return t.config.BaseURL + "/systemone"
}

func (t *OutboundTransformer) TransformResponse(
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

	if !json.Valid(httpResp.Body) {
		return nil, fmt.Errorf("failed to unmarshal systemone response: invalid json")
	}

	body := httpResp.Body

	// Some deployments (e.g. Cloudflare Workers AI) wrap the SystemOne payload
	// in their own universal envelope: {"success": true, "result": {...}}.
	// The envelope keys do not collide with the SystemOne fields, so decoding
	// the raw body directly would silently yield a zero-valued response.
	var envelope systemOneWireEnvelope
	if err := json.Unmarshal(httpResp.Body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal systemone response envelope: %w", err)
	}

	// The success flag decides whether the body is an envelope at all. Checking
	// presence rather than the decoded value keeps "success": null an envelope,
	// so it is rejected instead of falling through to the plain SystemOne path
	// and silently returning an empty answer.
	envelopePresent := len(envelope.Success) > 0
	if envelopePresent {
		success, ok := systemOneEnvelopeSuccess(envelope.Success)
		if !ok {
			return nil, systemOneEnvelopeError(
				http.StatusBadGateway,
				"systemone response envelope has an invalid success flag",
			)
		}

		if !success {
			msg := systemOneEnvelopeErrorMessage(&envelope)
			if msg == "" {
				msg = "upstream request failed"
			}

			return nil, systemOneEnvelopeError(http.StatusBadGateway, msg)
		}

		if systemOneIsEmptyJSON(envelope.Result) {
			return nil, systemOneEnvelopeError(
				http.StatusBadGateway,
				"systemone response envelope is missing result",
			)
		}

		if !json.Valid(envelope.Result) {
			return nil, fmt.Errorf("failed to unmarshal systemone response: invalid result json")
		}

		body = envelope.Result
	}

	var wireResp systemOneWireResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal systemone response: %w", err)
	}

	// An envelope that reports success still has to carry a SystemOne payload;
	// an empty or unrelated result would otherwise surface as an empty answer
	// instead of an upstream failure.
	if envelopePresent && wireResp.Model == "" && len(wireResp.Answers) == 0 {
		return nil, systemOneEnvelopeError(
			http.StatusBadGateway,
			"systemone response envelope result is empty",
		)
	}

	llmResp := &llm.Response{
		Model:       wireResp.Model,
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
		SystemOne: &llm.SystemOneResponse{
			Answers: wireResp.Answers,
		},
	}

	if wireResp.Usage != nil {
		llmResp.Usage = &llm.Usage{
			PromptTokens:     wireResp.Usage.InputTokens,
			CompletionTokens: wireResp.Usage.OutputTokens,
			TotalTokens:      wireResp.Usage.InputTokens + wireResp.Usage.OutputTokens,
		}
	}

	return llmResp, nil
}

// systemOneIsEmptyJSON reports whether a raw JSON value is absent or explicitly null.
func systemOneIsEmptyJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))

	return trimmed == "" || trimmed == "null"
}

// systemOneEnvelopeSuccess decodes the envelope success flag. ok is false when
// the flag is not a JSON boolean, including an explicit null or a string, so a
// malformed envelope can be rejected instead of read as a plain response.
func systemOneEnvelopeSuccess(raw json.RawMessage) (success bool, ok bool) {
	if systemOneIsEmptyJSON(raw) {
		return false, false
	}

	if err := json.Unmarshal(raw, &success); err != nil {
		return false, false
	}

	return success, true
}

// systemOneEnvelopeErrorMessage extracts a human-readable message from an error
// envelope, supporting string, object-with-message and array-of-objects shapes.
func systemOneEnvelopeErrorMessage(env *systemOneWireEnvelope) string {
	for _, raw := range []json.RawMessage{env.Error, env.Errors, env.Messages} {
		if systemOneIsEmptyJSON(raw) {
			continue
		}

		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			if msg := strings.TrimSpace(string(raw)); msg != "" {
				return msg
			}

			continue
		}

		if msg := systemOneMessageFromValue(value); msg != "" {
			return msg
		}
	}

	return ""
}

func systemOneMessageFromValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		if msg, ok := v["message"].(string); ok && msg != "" {
			return msg
		}

		if msg, ok := v["detail"].(string); ok && msg != "" {
			return msg
		}
	case []any:
		messages := make([]string, 0, len(v))
		for _, item := range v {
			if msg := systemOneMessageFromValue(item); msg != "" {
				messages = append(messages, msg)
			}
		}

		return strings.Join(messages, "; ")
	}

	return ""
}

func systemOneEnvelopeError(statusCode int, message string) *llm.ResponseError {
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}
	if message == "" {
		message = "systemone upstream error"
	}

	return &llm.ResponseError{
		StatusCode: statusCode,
		Detail: llm.ErrorDetail{
			Message: message,
			Type:    "api_error",
		},
	}
}

func (t *OutboundTransformer) TransformError(
	ctx context.Context,
	httpErr *httpclient.Error,
) *llm.ResponseError {
	if httpErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	var wireErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(httpErr.Body, &wireErr); err == nil && wireErr.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: httpErr.StatusCode,
			Cause:      httpErr,
			Detail: llm.ErrorDetail{
				Message: wireErr.Error.Message,
				Type:    wireErr.Error.Type,
				Code:    wireErr.Error.Code,
			},
		}
	}

	return &llm.ResponseError{
		StatusCode: httpErr.StatusCode,
		Cause:      httpErr,
		Detail: llm.ErrorDetail{
			Message: string(httpErr.Body),
			Type:    "api_error",
		},
	}
}

func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	req *httpclient.Request,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("systemone does not support streaming")
}

func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context, _ *httpclient.Request,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("systemone does not support streaming")
}
