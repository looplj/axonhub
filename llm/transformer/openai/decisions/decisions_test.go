package decisions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
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
		{name: "empty string input", body: `{"model":"gpt-6-luna","input":"","questions":[{}]}`},
		{name: "empty array input", body: `{"model":"gpt-6-luna","input":[],"questions":[{}]}`},
		{name: "empty object input", body: `{"model":"gpt-6-luna","input":{},"questions":[{}]}`},
		{name: "numeric input", body: `{"model":"gpt-6-luna","input":42,"questions":[{}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{Body: []byte(test.body)})
			require.Error(t, err)
			require.True(t, errors.Is(err, transformer.ErrInvalidRequest))
		})
	}
}

func TestDecisionsAcceptsSupportedInputForms(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "string", input: `"text"`},
		{name: "whitespace string", input: `"   "`},
		{name: "message with text", input: `[{"type":"message","role":"user","content":[{"type":"input_text","text":"text"}]}]`},
		{name: "message with image", input: `[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}]`},
		{name: "standalone text", input: `[{"type":"input_text","text":"text"}]`},
		{name: "standalone image", input: `[{"type":"input_image","image_url":"https://example.com/image.png"}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":"gpt-6-luna","input":%s,"questions":[{}]}`, test.input)
			_, err := NewInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
			require.NoError(t, err)
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

func TestDecisionsEmptySuccessfulResponsesReachEmptyResponseDetection(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			response, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
				StatusCode: status,
				Body:       nil,
			})
			require.NoError(t, err)
			require.NotNil(t, response.Decisions)
			require.Empty(t, response.Decisions.Answers)
		})
	}
}

func TestDecisionsRedirectIsAnUpstreamStatusError(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	rawBody := []byte(`{"error":{"message":"redirect body","type":"redirect_error"}}`)
	_, err = outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusTemporaryRedirect,
		Body:       rawBody,
		Headers:    http.Header{"Content-Type": []string{"application/problem+json"}},
	})

	var responseErr *llm.ResponseError
	require.ErrorAs(t, err, &responseErr)
	require.Equal(t, http.StatusTemporaryRedirect, responseErr.StatusCode)
	require.Equal(t, "redirect body", responseErr.Detail.Message)
	var httpErr *httpclient.Error
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, string(rawBody), string(httpErr.Body))
	require.Equal(t, "application/problem+json", httpErr.Headers.Get("Content-Type"))
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

func TestDecisionsOutboundURLRules(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{name: "default host", baseURL: "https://api.openai.com", expected: "https://api.openai.com/v1/decisions"},
		{name: "versioned host", baseURL: "https://api.openai.com/v1", expected: "https://api.openai.com/v1/decisions"},
		{name: "trailing slash", baseURL: "https://api.openai.com/", expected: "https://api.openai.com/v1/decisions"},
		{name: "single marker", baseURL: "https://relay.example/api#", expected: "https://relay.example/api/decisions"},
		{name: "explicit path", baseURL: "https://relay.example/api/v2", path: "/custom/decisions", expected: "https://relay.example/api/v2/custom/decisions"},
		{name: "raw marker", baseURL: "https://relay.example/decisions##", expected: "https://relay.example/decisions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given a Decisions endpoint configuration
			outbound, err := NewOutboundTransformerWithConfig(&Config{
				BaseURL:        test.baseURL,
				EndpointPath:   test.path,
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			})
			require.NoError(t, err)

			// When the outbound request is built
			request, err := outbound.TransformRequest(context.Background(), &llm.Request{
				Model:     "gpt-6-luna",
				Decisions: &llm.DecisionsRequest{Body: []byte(`{"model":"gpt-6-luna"}`)},
			})
			require.NoError(t, err)

			// Then the marker and version rules produce the expected URL
			require.Equal(t, test.expected, request.URL)
		})
	}
}
