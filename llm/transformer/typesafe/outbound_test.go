package typesafe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	cfg := &Config{
		BaseURL:        "https://api.typesafe.ai/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("sk-test-typesafe-key"),
	}
	outbound, err := NewOutboundTransformerWithConfig(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	llmReq := &llm.Request{
		Model:       "jev-latest",
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
		SystemOne: &llm.SystemOneRequest{
			State: "Sample state",
			Questions: map[string]llm.SystemOneQuestion{
				"check": {Type: "noul", Instructions: "True?"},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(ctx, llmReq)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, httpReq.Method)
	require.Equal(t, "https://api.typesafe.ai/v1/systemone", httpReq.URL)
	require.Equal(t, "sk-test-typesafe-key", httpReq.Auth.APIKey)
	require.Contains(t, string(httpReq.Body), `"state":"Sample state"`)
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	cfg := &Config{
		BaseURL:        "https://api.typesafe.ai/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("sk-test-typesafe-key"),
	}
	outbound, err := NewOutboundTransformerWithConfig(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	rawResp := []byte(`{
		"model": "jev-latest",
		"answers": {
			"check": {
				"type": "noul",
				"noul": 0.945
			}
		},
		"usage": {
			"input_tokens": 120,
			"output_tokens": 0
		}
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       rawResp,
		Request: &httpclient.Request{
			APIFormat: string(llm.APIFormatTypeSafeSystemOne),
		},
	}

	llmResp, err := outbound.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.Equal(t, "jev-latest", llmResp.Model)
	require.Equal(t, llm.RequestTypeSystemOne, llmResp.RequestType)
	require.NotNil(t, llmResp.SystemOne)
	require.NotNil(t, llmResp.Usage)
	require.Equal(t, int64(120), llmResp.Usage.PromptTokens)
	require.Equal(t, int64(0), llmResp.Usage.CompletionTokens)
}

func TestOutboundTransformer_TransformResponse_NumericPrecision(t *testing.T) {
	cfg := &Config{
		BaseURL:        "https://api.typesafe.ai/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("sk-test-typesafe-key"),
	}
	outbound, err := NewOutboundTransformerWithConfig(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	// JSON containing 64-bit snowflake in extra/legend and exact probabilities
	rawResp := []byte(`{
		"model": "jev-latest",
		"answers": {
			"quality": {
				"type": "score",
				"score": 4.5,
				"probabilities": {"1": 0.05, "2": 0.95},
				"legend": {"level_id": 9007199254740999}
			}
		},
		"usage": {
			"input_tokens": 100,
			"output_tokens": 0
		}
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       rawResp,
		Request: &httpclient.Request{
			APIFormat: string(llm.APIFormatTypeSafeSystemOne),
		},
	}

	llmResp, err := outbound.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp.SystemOne)
	ans := llmResp.SystemOne.Answers["quality"]
	require.Equal(t, "score", ans.Type)
	require.NotNil(t, ans.Score)
	require.Equal(t, 4.5, *ans.Score)

	// Check that probabilities and legend preserve string/exact representation
	probs, ok := ans.Probabilities.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "0.95", fmt.Sprintf("%v", probs["2"]))

	legend, ok := ans.Legend.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "9007199254740999", fmt.Sprintf("%v", legend["level_id"]))
}

func TestOutboundTransformer_TransformError_PreservesCause(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.typesafe.ai/v1", "sk-test")
	require.NoError(t, err)

	ctx := context.Background()
	origHttpErr := &httpclient.Error{
		StatusCode: http.StatusTooManyRequests,
		Status:     "429 Too Many Requests",
		Body:       []byte(`{"error":{"message":"RPM limit exceeded","type":"rate_limit_error","code":"rate_limit"}}`),
	}

	respErr := outbound.TransformError(ctx, origHttpErr)
	require.NotNil(t, respErr)
	require.Equal(t, http.StatusTooManyRequests, respErr.StatusCode)
	require.Equal(t, "RPM limit exceeded", respErr.Detail.Message)
	require.Equal(t, origHttpErr, respErr.Cause)

	// Verify errors.As properly unwraps and extracts origHttpErr
	var targetHttpErr *httpclient.Error
	require.True(t, errors.As(respErr, &targetHttpErr))
	require.Equal(t, http.StatusTooManyRequests, targetHttpErr.StatusCode)
}

func TestOutboundTransformer_TransformResponse_InvalidJSON(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.typesafe.ai/v1", "sk-test")
	require.NoError(t, err)

	ctx := context.Background()
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"model": "jev-latest"} trailing malformed data`),
		Request: &httpclient.Request{
			APIFormat: string(llm.APIFormatTypeSafeSystemOne),
		},
	}

	_, err = outbound.TransformResponse(ctx, httpResp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid json")
}
