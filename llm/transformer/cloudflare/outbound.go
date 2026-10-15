package cloudflare

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

// Config configures the Workers AI System One outbound. BaseURL is the
// account-scoped Workers AI base, e.g.
// https://api.cloudflare.com/client/v4/accounts/<account_id>/ai.
type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
	// EndpointPath optionally replaces the default "/run" route prefix; the
	// model id is always appended because Workers AI routes by model.
	EndpointPath string `json:"endpoint_path,omitempty"`
}

// SystemOneOutboundTransformer sends System One decision requests to Workers AI
// decision models (e.g. @cf/cloudflare/clef) on the native /run/{model} route.
type SystemOneOutboundTransformer struct {
	config *Config
}

func NewSystemOneOutboundTransformerWithConfig(config *Config) (*SystemOneOutboundTransformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required")
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	// The OpenAI-compatible surface lives under {base}/v1, so tolerate a base URL
	// that already carries it: native runs are addressed from the account base.
	baseURL := transformer.NormalizeBaseURL(config.BaseURL, "")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	return &SystemOneOutboundTransformer{
		config: &Config{
			BaseURL:        baseURL,
			APIKeyProvider: config.APIKeyProvider,
			EndpointPath:   config.EndpointPath,
		},
	}, nil
}

func (t *SystemOneOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatTypeSafeSystemOne
}

// AllowPassThroughBody disables raw body reuse: Workers AI requires the model
// selector in the body, which differs from the model id carried by the client.
func (t *SystemOneOutboundTransformer) AllowPassThroughBody(context.Context, *llm.Request, *httpclient.Request) bool {
	return false
}

func (t *SystemOneOutboundTransformer) TransformRequest(
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
		Model:     modelSelector(llmReq.Model),
		State:     llmReq.SystemOne.State,
		Questions: llmReq.SystemOne.Questions,
	}

	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal systemone request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     t.buildURL(llmReq.Model),
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKeyProvider.Get(ctx),
		},
		RequestType: string(llm.RequestTypeSystemOne),
		APIFormat:   string(llm.APIFormatTypeSafeSystemOne),
	}, nil
}

func (t *SystemOneOutboundTransformer) buildURL(model string) string {
	prefix := "/run"
	if t.config.EndpointPath != "" {
		prefix = strings.TrimRight(t.config.EndpointPath, "/")
	}

	return t.config.BaseURL + prefix + "/" + model
}

// modelSelector strips the "@cf/<author>/" prefix from a Workers AI model id,
// e.g. "@cf/cloudflare/clef" -> "clef", as required by the decision model schema.
func modelSelector(model string) string {
	rest, ok := strings.CutPrefix(model, "@cf/")
	if !ok {
		return model
	}

	if _, selector, ok := strings.Cut(rest, "/"); ok && selector != "" {
		return selector
	}

	return model
}

func (t *SystemOneOutboundTransformer) TransformResponse(
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

	payload := httpResp.Body

	var env envelope
	if err := json.Unmarshal(httpResp.Body, &env); err == nil {
		if result := bytes.TrimSpace(env.Result); len(result) > 0 && result[0] == '{' {
			payload = result
		}
	}

	var wireResp systemOneWireResponse
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal systemone response: %w", err)
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

func (t *SystemOneOutboundTransformer) TransformError(
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

	var env envelope
	if err := json.Unmarshal(httpErr.Body, &env); err == nil {
		messages := make([]string, 0, len(env.Errors))
		code := ""
		for _, e := range env.Errors {
			if e.Message == "" {
				continue
			}
			messages = append(messages, e.Message)
			if code == "" {
				code = strings.Trim(string(e.Code), `"`)
			}
		}

		if len(messages) > 0 {
			return &llm.ResponseError{
				StatusCode: httpErr.StatusCode,
				Cause:      httpErr,
				Detail: llm.ErrorDetail{
					Message: strings.Join(messages, "; "),
					Type:    "api_error",
					Code:    code,
				},
			}
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

func (t *SystemOneOutboundTransformer) TransformStream(
	ctx context.Context,
	req *httpclient.Request,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("systemone does not support streaming")
}

func (t *SystemOneOutboundTransformer) AggregateStreamChunks(
	ctx context.Context, _ *httpclient.Request,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("systemone does not support streaming")
}
