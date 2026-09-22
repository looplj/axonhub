package typesafe

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestSystemOneInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewSystemOneInboundTransformer()
	ctx := context.Background()

	validBody := []byte(`{
		"model": "jev-latest",
		"state": {
			"large_int": 9007199254740993
		},
		"questions": {
			"urgent": {
				"type": "noul",
				"instructions": "Is this urgent?"
			}
		}
	}`)

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		Body:   validBody,
	}

	llmReq, err := transformer.TransformRequest(ctx, httpReq)
	require.NoError(t, err)
	require.Equal(t, "jev-latest", llmReq.Model)
	require.Equal(t, llm.RequestTypeSystemOne, llmReq.RequestType)
	require.Equal(t, llm.APIFormatTypeSafeSystemOne, llmReq.APIFormat)
	require.NotNil(t, llmReq.SystemOne)

	// Verify large int is preserved via json.Number
	stateMap, ok := llmReq.SystemOne.State.(map[string]any)
	require.True(t, ok)
	num, ok := stateMap["large_int"].(json.Number)
	require.True(t, ok)
	require.Equal(t, "9007199254740993", num.String())

	require.Contains(t, llmReq.SystemOne.Questions, "urgent")

	// Missing model should fail
	invalidBody := []byte(`{"state": "foo"}`)
	_, err = transformer.TransformRequest(ctx, &httpclient.Request{Method: http.MethodPost, Body: invalidBody})
	require.Error(t, err)

	// Stream: true should fail with ErrInvalidRequest
	streamBody := []byte(`{
		"model": "jev-latest",
		"stream": true,
		"state": "foo",
		"questions": {
			"urgent": {
				"type": "noul",
				"instructions": "Is this urgent?"
			}
		}
	}`)
	_, err = transformer.TransformRequest(ctx, &httpclient.Request{Method: http.MethodPost, Body: streamBody})
	require.Error(t, err)
}

func TestSystemOneInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewSystemOneInboundTransformer()
	ctx := context.Background()

	prob := 0.98
	llmResp := &llm.Response{
		Model: "jev-latest",
		SystemOne: &llm.SystemOneResponse{
			Answers: map[string]llm.SystemOneAnswer{
				"urgent": {
					Type: "noul",
					Noul: &prob,
				},
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 1,
		},
	}

	httpResp, err := transformer.TransformResponse(ctx, llmResp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	require.Contains(t, string(httpResp.Body), `"noul":0.98`)
	require.Contains(t, string(httpResp.Body), `"input_tokens":10`)
	require.Contains(t, string(httpResp.Body), `"output_tokens":1`)
}
