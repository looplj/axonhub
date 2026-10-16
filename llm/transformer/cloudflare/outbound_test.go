package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

var (
	_ transformer.Outbound                  = (*SystemOneOutboundTransformer)(nil)
	_ transformer.PassThroughBodyPolicy     = (*SystemOneOutboundTransformer)(nil)
	_ transformer.PassThroughResponsePolicy = (*SystemOneOutboundTransformer)(nil)
)

const testBaseURL = "https://api.cloudflare.com/client/v4/accounts/<account_id>/ai"

func newTestOutbound(t *testing.T, baseURL string) *SystemOneOutboundTransformer {
	t.Helper()

	outbound, err := NewSystemOneOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	return outbound
}

func newSystemOneRequest(model string) *llm.Request {
	return &llm.Request{
		Model:       model,
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
		SystemOne: &llm.SystemOneRequest{
			State: map[string]any{"ticket": "Refund request"},
			Questions: map[string]llm.SystemOneQuestion{
				"check": {Type: "noul", Instructions: "Is this a refund?"},
			},
		},
	}
}

func TestNewSystemOneOutboundTransformerWithConfig_Validation(t *testing.T) {
	_, err := NewSystemOneOutboundTransformerWithConfig(nil)
	require.Error(t, err)

	_, err = NewSystemOneOutboundTransformerWithConfig(&Config{BaseURL: testBaseURL})
	require.Error(t, err)

	_, err = NewSystemOneOutboundTransformerWithConfig(&Config{APIKeyProvider: auth.NewStaticKeyProvider("test-key")})
	require.Error(t, err)
}

func TestSystemOneOutboundTransformer_TransformRequest_URL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		endpointPath string
		model        string
		want         string
	}{
		{
			name:    "account base",
			baseURL: testBaseURL,
			model:   "@cf/cloudflare/clef",
			want:    testBaseURL + "/run/@cf/cloudflare/clef",
		},
		{
			name:    "trailing slash",
			baseURL: testBaseURL + "/",
			model:   "@cf/cloudflare/clef-flash",
			want:    testBaseURL + "/run/@cf/cloudflare/clef-flash",
		},
		{
			name:    "base with v1 suffix",
			baseURL: testBaseURL + "/v1",
			model:   "@cf/cloudflare/clef-omni",
			want:    testBaseURL + "/run/@cf/cloudflare/clef-omni",
		},
		{
			name:    "base with v1 suffix and trailing slash",
			baseURL: testBaseURL + "/v1/",
			model:   "@cf/cloudflare/clef",
			want:    testBaseURL + "/run/@cf/cloudflare/clef",
		},
		{
			name:         "custom endpoint path",
			baseURL:      testBaseURL,
			endpointPath: "/custom/run/",
			model:        "@cf/cloudflare/clef",
			want:         testBaseURL + "/custom/run/@cf/cloudflare/clef",
		},
		{
			name:         "custom endpoint path without leading slash",
			baseURL:      testBaseURL,
			endpointPath: "custom/run",
			model:        "@cf/cloudflare/clef",
			want:         testBaseURL + "/custom/run/@cf/cloudflare/clef",
		},
		{
			name:         "root endpoint path",
			baseURL:      testBaseURL,
			endpointPath: "/",
			model:        "@cf/cloudflare/clef",
			want:         testBaseURL + "/@cf/cloudflare/clef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound, err := NewSystemOneOutboundTransformerWithConfig(&Config{
				BaseURL:        tt.baseURL,
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
				EndpointPath:   tt.endpointPath,
			})
			require.NoError(t, err)

			req, err := outbound.TransformRequest(context.Background(), newSystemOneRequest(tt.model))
			require.NoError(t, err)
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, tt.want, req.URL)
			require.Equal(t, "bearer", req.Auth.Type)
			require.Equal(t, "test-key", req.Auth.APIKey)
			require.Equal(t, string(llm.APIFormatTypeSafeSystemOne), req.APIFormat)
		})
	}
}

func TestSystemOneOutboundTransformer_TransformRequest_ModelSelector(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{model: "@cf/cloudflare/clef", want: "clef"},
		{model: "@cf/cloudflare/clef-flash", want: "clef-flash"},
		{model: "@cf/cloudflare/clef-omni", want: "clef-omni"},
		{model: "clef", want: "clef"},
	}

	outbound := newTestOutbound(t, testBaseURL)

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			req, err := outbound.TransformRequest(context.Background(), newSystemOneRequest(tt.model))
			require.NoError(t, err)

			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(req.Body, &body))
			require.JSONEq(t, `"`+tt.want+`"`, string(body["model"]))
			require.JSONEq(t, `{"ticket":"Refund request"}`, string(body["state"]))
			require.JSONEq(t, `{"check":{"type":"noul","instructions":"Is this a refund?"}}`, string(body["questions"]))
			require.NotContains(t, body, "input")
		})
	}
}

func TestSystemOneOutboundTransformer_TransformRequest_Invalid(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)

	_, err := outbound.TransformRequest(context.Background(), nil)
	require.Error(t, err)

	_, err = outbound.TransformRequest(context.Background(), &llm.Request{Model: "@cf/cloudflare/clef"})
	require.Error(t, err)

	_, err = outbound.TransformRequest(context.Background(), newSystemOneRequest(""))
	require.Error(t, err)
}

func TestSystemOneOutboundTransformer_DisallowsPassThroughBody(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)
	require.False(t, outbound.AllowPassThroughBody(context.Background(), newSystemOneRequest("@cf/cloudflare/clef"), nil))
}

func TestSystemOneOutboundTransformer_DisallowsPassThroughResponse(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)
	require.False(t, outbound.AllowPassThroughResponse(context.Background(), newSystemOneRequest("@cf/cloudflare/clef"), nil))
}

func TestSystemOneOutboundTransformer_TransformResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "cloudflare envelope",
			body: `{
				"success": true,
				"result": {
					"model": "clef",
					"answers": {"check": {"type": "noul", "noul": 0.945}},
					"usage": {"input_tokens": 120, "output_tokens": 3}
				},
				"errors": [],
				"messages": []
			}`,
		},
		{
			name: "flat system one",
			body: `{
				"model": "clef",
				"answers": {"check": {"type": "noul", "noul": 0.945}},
				"usage": {"input_tokens": 120, "output_tokens": 3}
			}`,
		},
	}

	outbound := newTestOutbound(t, testBaseURL)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(tt.body),
			})
			require.NoError(t, err)
			require.Equal(t, "clef", resp.Model)
			require.Equal(t, llm.RequestTypeSystemOne, resp.RequestType)
			require.Equal(t, llm.APIFormatTypeSafeSystemOne, resp.APIFormat)
			require.NotNil(t, resp.SystemOne)
			require.Len(t, resp.SystemOne.Answers, 1)
			require.Equal(t, "noul", resp.SystemOne.Answers["check"].Type)
			require.NotNil(t, resp.SystemOne.Answers["check"].Noul)
			require.InDelta(t, 0.945, *resp.SystemOne.Answers["check"].Noul, 1e-9)
			require.NotNil(t, resp.Usage)
			require.Equal(t, int64(120), resp.Usage.PromptTokens)
			require.Equal(t, int64(3), resp.Usage.CompletionTokens)
			require.Equal(t, int64(123), resp.Usage.TotalTokens)
		})
	}
}

func TestSystemOneOutboundTransformer_TransformResponse_EnvelopeFailure(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)

	// Workers AI reports failures inside a 2xx envelope; an unwrapped body would
	// otherwise decode into an empty answer set and look successful.
	t.Run("success false with 200", func(t *testing.T) {
		_, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"success":false,"errors":[{"code":5006,"message":"AiError: Bad input"}],"messages":[],"result":null}`),
		})
		require.Error(t, err)

		var respErr *llm.ResponseError
		require.ErrorAs(t, err, &respErr)
		require.Equal(t, http.StatusBadGateway, respErr.StatusCode)
		require.Equal(t, "AiError: Bad input", respErr.Detail.Message)
		require.Equal(t, "5006", respErr.Detail.Code)
	})

	t.Run("success false with 4xx keeps status", func(t *testing.T) {
		_, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       []byte(`{"success":false,"errors":[{"code":3040,"message":"Capacity temporarily exceeded"}],"messages":[],"result":null}`),
		})
		require.Error(t, err)

		var respErr *llm.ResponseError
		require.ErrorAs(t, err, &respErr)
		require.Equal(t, http.StatusTooManyRequests, respErr.StatusCode)
		require.Equal(t, "Capacity temporarily exceeded", respErr.Detail.Message)
		require.Equal(t, "3040", respErr.Detail.Code)
	})

	t.Run("errors without success flag", func(t *testing.T) {
		_, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"errors":[{"code":7001,"message":"GET not supported for requested URI."}],"result":null}`),
		})
		require.Error(t, err)

		var respErr *llm.ResponseError
		require.ErrorAs(t, err, &respErr)
		require.Equal(t, http.StatusBadGateway, respErr.StatusCode)
	})

	t.Run("success without result", func(t *testing.T) {
		_, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"success":true,"errors":[],"messages":[]}`),
		})
		require.ErrorContains(t, err, "envelope has no result")
	})

	t.Run("result is not an object", func(t *testing.T) {
		_, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"success":true,"result":null,"errors":[],"messages":[]}`),
		})
		require.ErrorContains(t, err, "unexpected result")
	})
}

func TestSystemOneOutboundTransformer_TransformResponse_NoAnswers(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "envelope result without answers",
			body: `{"success":true,"result":{"model":"clef"}}`,
		},
		{
			name: "envelope result with empty answers",
			body: `{"success":true,"result":{"model":"clef","answers":{}}}`,
		},
		{
			name: "flat body without answers",
			body: `{"model":"clef","usage":{"input_tokens":120,"output_tokens":3}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(tt.body),
			})
			require.ErrorContains(t, err, "systemone response has no answers")
			require.Nil(t, resp)
		})
	}
}

func TestSystemOneOutboundTransformer_TransformResponse_Invalid(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)

	_, err := outbound.TransformResponse(context.Background(), nil)
	require.Error(t, err)

	_, err = outbound.TransformResponse(context.Background(), &httpclient.Response{StatusCode: http.StatusOK})
	require.Error(t, err)

	_, err = outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"result":`),
	})
	require.Error(t, err)
}

func TestSystemOneOutboundTransformer_TransformError(t *testing.T) {
	outbound := newTestOutbound(t, testBaseURL)

	t.Run("cloudflare envelope", func(t *testing.T) {
		body := []byte(`{"success":false,"errors":[{"code":7003,"message":"Could not route to /client/v4/accounts/<account_id>/ai/run/@cf/cloudflare/unknown"}],"messages":[],"result":null}`)

		_, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
			StatusCode: http.StatusBadRequest,
			Body:       body,
		})
		require.Error(t, err)

		var respErr *llm.ResponseError
		require.ErrorAs(t, err, &respErr)
		require.Equal(t, http.StatusBadRequest, respErr.StatusCode)
		require.Equal(t, "Could not route to /client/v4/accounts/<account_id>/ai/run/@cf/cloudflare/unknown", respErr.Detail.Message)
		require.Equal(t, "7003", respErr.Detail.Code)
		require.Equal(t, "api_error", respErr.Detail.Type)
	})

	t.Run("multiple envelope errors", func(t *testing.T) {
		respErr := outbound.TransformError(context.Background(), &httpclient.Error{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"},{"code":"9109","message":"Invalid access token"}]}`),
		})
		require.Equal(t, http.StatusUnauthorized, respErr.StatusCode)
		require.Equal(t, "Authentication error; Invalid access token", respErr.Detail.Message)
		require.Equal(t, "10000", respErr.Detail.Code)
	})

	t.Run("raw body fallback", func(t *testing.T) {
		respErr := outbound.TransformError(context.Background(), &httpclient.Error{
			StatusCode: http.StatusBadGateway,
			Body:       []byte("upstream unavailable"),
		})
		require.Equal(t, http.StatusBadGateway, respErr.StatusCode)
		require.Equal(t, "upstream unavailable", respErr.Detail.Message)
		require.Equal(t, "api_error", respErr.Detail.Type)
	})

	t.Run("openai style error body", func(t *testing.T) {
		respErr := outbound.TransformError(context.Background(), &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error":{"message":"upstream boom","type":"invalid_request_error"}}`),
		})
		require.Equal(t, http.StatusBadRequest, respErr.StatusCode)
		require.Contains(t, respErr.Detail.Message, "upstream boom")
	})

	t.Run("nil error", func(t *testing.T) {
		respErr := outbound.TransformError(context.Background(), nil)
		require.Equal(t, http.StatusInternalServerError, respErr.StatusCode)
	})
}
