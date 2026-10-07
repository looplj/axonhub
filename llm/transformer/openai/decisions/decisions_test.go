package decisions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestDecisionsClientShapedRoundTripPreservesRawFieldsAndUsage(t *testing.T) {
	rawRequest, err := os.ReadFile("testdata/client_request.json")
	require.NoError(t, err)
	rawResponse, err := os.ReadFile("testdata/provider_response.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	request, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Method:  http.MethodPost,
		Body:    rawRequest,
		Headers: http.Header{"Content-Type": []string{"application/json"}},
	})
	require.NoError(t, err)
	require.Equal(t, llm.RequestTypeDecisions, request.RequestType)
	require.Equal(t, "gpt-6-luna", request.Decisions.Model)
	require.Len(t, request.Decisions.Questions, 1)
	var question struct {
		Choices []struct {
			Value json.RawMessage `json:"value"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(request.Decisions.Questions[0], &question))
	require.Equal(t, `"billing"`, string(question.Choices[0].Value))
	require.Equal(t, `true`, string(question.Choices[1].Value))

	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	providerRequest, err := outbound.TransformRequest(context.Background(), request)
	require.NoError(t, err)
	require.JSONEq(t, string(rawRequest), string(providerRequest.Body))

	providerResponse, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       rawResponse,
		Request:    providerRequest,
	})
	require.NoError(t, err)
	require.Equal(t, int64(120), providerResponse.Usage.PromptTokens)
	require.Equal(t, int64(0), providerResponse.Usage.CompletionTokens)
	require.Equal(t, int64(0), providerResponse.Usage.TotalTokens)
	require.Len(t, providerResponse.Decisions.Answers, 2)
	var refusal struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(providerResponse.Decisions.Answers[1], &refusal))
	require.Equal(t, "refusal", refusal.Type)
	require.Nil(t, providerResponse.Usage.PromptTokensDetails)
	require.Nil(t, providerResponse.Usage.CompletionTokensDetails)

	clientResponse, err := inbound.TransformResponse(context.Background(), providerResponse)
	require.NoError(t, err)
	require.JSONEq(t, string(rawResponse), string(clientResponse.Body))
}

func TestDecisionsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"model":`},
		{name: "missing model", body: `{"input":"text","questions":[{}]}`},
		{name: "missing questions", body: `{"model":"gpt-6-luna","input":"text"}`},
		{name: "null input", body: `{"model":"gpt-6-luna","input":null,"questions":[{}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{Body: []byte(test.body)})
			require.Error(t, err)
			require.True(t, errors.Is(err, transformer.ErrInvalidRequest))
		})
	}
}

func TestDecisionsStreamIsRejected(t *testing.T) {
	_, err := NewInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{"model":"gpt-6-luna","input":"text","questions":[{}],"stream":true}`),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest))
}

func TestDecisionsRejectsTrailingJSONData(t *testing.T) {
	rawRequest, err := os.ReadFile("testdata/trailing_data.json")
	require.NoError(t, err)

	_, err = NewInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{Body: rawRequest})
	require.Error(t, err)
	require.True(t, errors.Is(err, transformer.ErrInvalidRequest))
}

func TestDecisionsUsageCanBeNull(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	response, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"model":"gpt-6-luna","answers":[],"usage":null}`),
	})
	require.NoError(t, err)
	require.Nil(t, response.Usage)
	require.Nil(t, response.Decisions.Usage)
}

func TestDecisionsErrorFixturePreservesClassification(t *testing.T) {
	rawError, err := os.ReadFile("testdata/provider_error.json")
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	_, err = outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       rawError,
	})
	require.Error(t, err)
	var responseErr *llm.ResponseError
	require.ErrorAs(t, err, &responseErr)
	require.Equal(t, http.StatusTooManyRequests, responseErr.StatusCode)
	require.Equal(t, "decision rate limit", responseErr.Detail.Message)
	require.Equal(t, "rate_limit_error", responseErr.Detail.Type)
}
