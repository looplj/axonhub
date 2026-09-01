package responses

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func TestIsInvalidEncryptedContentError(t *testing.T) {
	native := &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"code":"invalid_encrypted_content"}}`),
	}
	require.Equal(t, encryptedContentFailureInvalid, detectEncryptedContentFailure(native, nil))

	// Some gateways put the provider JSON in a string-valued top-level message.
	wrapper := map[string]any{
		"code":    -4201,
		"message": `{"error":{"message":"could not be verified","code":"invalid_encrypted_content"}}`,
	}
	wrapperBody, err := json.Marshal(wrapper)
	require.NoError(t, err)
	require.Equal(t, encryptedContentFailureInvalid, detectEncryptedContentFailure(&httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       wrapperBody,
	}, nil))
	require.Equal(t, encryptedContentFailureInvalid, detectEncryptedContentFailure(&httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"code":-4201,"message":"invalid_encrypted_content: could not be verified"}`),
	}, nil))

	require.Equal(t, encryptedContentFailureNone, detectEncryptedContentFailure(&httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"code":"invalid_request_error"}}`),
	}, nil))
}

func TestDetectEncryptedContentFailureCrossResource(t *testing.T) {
	require.Equal(t, encryptedContentFailureCrossResource, detectEncryptedContentFailure(&httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"code":-4201,"message":"{\"error\":{\"message\":\"The requested item was created under a different Azure OpenAI resource. Use the same resource that created the item to access it.\",\"type\":\"invalid_request_error\",\"code\":null}}"}`),
	}, nil))
}

func TestStripEncryptedReasoningContent(t *testing.T) {
	input := []byte(`{
		"model":"gpt-5.6",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"keep me"}]},
			{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"visible summary"}],"encrypted_content":"gAAAA_reasoning"},
			{"type":"function_call_output","output":[
				{"type":"input_text","text":"keep output"},
				{"type":"encrypted_content","encrypted_content":"gAAAA_function"},
				{"type":"input_text","text":"keep order"}
			]}
		]
	}`)

	stripped, ok := stripAccountBoundResponseItems(input, false)
	require.True(t, ok)
	require.Equal(t, input, []byte(`{
		"model":"gpt-5.6",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"keep me"}]},
			{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"visible summary"}],"encrypted_content":"gAAAA_reasoning"},
			{"type":"function_call_output","output":[
				{"type":"input_text","text":"keep output"},
				{"type":"encrypted_content","encrypted_content":"gAAAA_function"},
				{"type":"input_text","text":"keep order"}
			]}
		]
	}`), "strip must not mutate the original request")
	require.False(t, gjson.GetBytes(stripped, "input.1.encrypted_content").Exists())
	require.Equal(t, "rs_1", gjson.GetBytes(stripped, "input.1.id").String())
	require.Equal(t, "visible summary", gjson.GetBytes(stripped, "input.1.summary.0.text").String())
	require.Equal(t, "keep me", gjson.GetBytes(stripped, "input.0.content.0.text").String())
	require.Equal(t, float64(2), gjson.GetBytes(stripped, "input.2.output.#").Float())
	require.Equal(t, "keep output", gjson.GetBytes(stripped, "input.2.output.0.text").String())
	require.Equal(t, "keep order", gjson.GetBytes(stripped, "input.2.output.1.text").String())
	require.NotContains(t, string(stripped), "gAAAA_")
}

func TestStripEncryptedReasoningContentNoOp(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"input":[{"type":"reasoning","encrypted_content":null}]}`),
		[]byte(`{"input":[{"type":"message","content":[]}]}`),
		[]byte(`{"input":"plain text"}`),
		[]byte(`not json`),
	}

	for _, input := range cases {
		stripped, ok := stripAccountBoundResponseItems(input, false)
		require.False(t, ok)
		require.Equal(t, input, stripped)
	}
}

func TestPrepareEncryptedContentRetryRequestCrossResource(t *testing.T) {
	request := &httpclient.Request{
		Method:                       http.MethodPost,
		APIFormat:                    llm.APIFormatOpenAIResponse.String(),
		RetryInvalidEncryptedContent: true,
		Body: []byte(`{"input":[
			{"id":"msg_1","type":"message","role":"user","content":[{"type":"input_text","text":"keep me"}]},
			{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"keep summary"}],"encrypted_content":"gAAAA_reasoning"},
			{"id":"fc_1","type":"function_call","call_id":"call_1","name":"tool","arguments":"{}"},
			{"id":"fco_1","type":"function_call_output","call_id":"call_1","output":[{"type":"encrypted_content","encrypted_content":"gAAAA_output"},{"type":"input_text","text":"keep output"}]}
		]}`),
	}
	err := &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"message":"The requested item was created under a different Azure OpenAI resource. Use the same resource that created the item to access it.","code":null}}`),
	}

	retry, ok := PrepareEncryptedContentRetryRequest(request, nil, err)
	require.True(t, ok)
	require.Equal(t, "rs_1", gjson.GetBytes(request.Body, "input.1.id").String())
	require.Equal(t, "gAAAA_reasoning", gjson.GetBytes(request.Body, "input.1.encrypted_content").String())
	require.NotContains(t, string(retry.Body), "gAAAA")
	require.False(t, gjson.GetBytes(retry.Body, "input.0.id").Exists())
	require.False(t, gjson.GetBytes(retry.Body, "input.1.id").Exists())
	require.False(t, gjson.GetBytes(retry.Body, "input.2.id").Exists())
	require.False(t, gjson.GetBytes(retry.Body, "input.3.id").Exists())
	require.Equal(t, "keep me", gjson.GetBytes(retry.Body, "input.0.content.0.text").String())
	require.Equal(t, "keep summary", gjson.GetBytes(retry.Body, "input.1.summary.0.text").String())
	recoveredCallID := gjson.GetBytes(retry.Body, "input.2.call_id").String()
	require.Equal(t, "call_recovered_1", recoveredCallID)
	require.NotEqual(t, "call_1", recoveredCallID)
	require.Equal(t, recoveredCallID, gjson.GetBytes(retry.Body, "input.3.call_id").String())
	require.Equal(t, "keep output", gjson.GetBytes(retry.Body, "input.3.output.0.text").String())
}

func TestPrepareEncryptedContentRetryRequest(t *testing.T) {
	request := &httpclient.Request{
		Method:                       http.MethodPost,
		APIFormat:                    llm.APIFormatOpenAIResponse.String(),
		RetryInvalidEncryptedContent: true,
		Headers:                      make(http.Header),
		Query:                        make(url.Values),
		Metadata:                     map[string]string{"existing": "value"},
		TransformerMetadata:          map[string]any{"existing": true},
		Auth:                         &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body:                         []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`),
		JSONBody:                     []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`),
	}
	err := &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"code":"invalid_encrypted_content"}}`),
	}

	retry, ok := PrepareEncryptedContentRetryRequest(request, nil, err)
	require.True(t, ok)
	require.NotSame(t, request, retry)
	require.Equal(t, request.Body, []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`))
	require.NotContains(t, string(retry.Body), "gAAAA")
	require.NotContains(t, string(retry.JSONBody), "gAAAA")

	retry.Headers.Set("X-Retry-Only", "true")
	retry.Query.Set("retry_only", "true")
	retry.Metadata["retry_only"] = "true"
	retry.TransformerMetadata["retry_only"] = true
	retry.Auth.APIKey = "retry-only"
	require.Empty(t, request.Headers.Get("X-Retry-Only"))
	require.Empty(t, request.Query.Get("retry_only"))
	require.Empty(t, request.Metadata["retry_only"])
	require.Nil(t, request.TransformerMetadata["retry_only"])
	require.Equal(t, "test-key", request.Auth.APIKey)

	disabled := *request
	disabled.RetryInvalidEncryptedContent = false
	_, ok = PrepareEncryptedContentRetryRequest(&disabled, nil, err)
	require.False(t, ok)

	_, ok = PrepareEncryptedContentRetryRequest(request, nil, &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"code":"rate_limit_exceeded"}}`),
	})
	require.False(t, ok)
}

func TestEncryptedContentRetryExecutorDoesNotRetryWhenDisabled(t *testing.T) {
	stub := &encryptedRetryExecutorStub{
		streamErrs: []error{&httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error":{"code":"invalid_encrypted_content"}}`),
		}},
	}

	executor := NewEncryptedContentRetryExecutor(stub)
	_, err := executor.DoStream(context.Background(), &httpclient.Request{
		APIFormat: llm.APIFormatOpenAIResponse.String(),
		Body:      []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`),
	})
	require.Error(t, err)
	require.Len(t, stub.streamRequests, 1)
}

func TestEncryptedContentRetryExecutorRetriesStreamingRequest(t *testing.T) {
	firstBody := []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`)
	stub := &encryptedRetryExecutorStub{
		streamErrs: []error{
			&httpclient.Error{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"error":{"code":"invalid_encrypted_content"}}`),
			},
		},
		streamResults: []streams.Stream[*httpclient.StreamEvent]{
			nil,
			streams.SliceStream([]*httpclient.StreamEvent{{Type: "response.completed", Data: []byte(`{"type":"response.completed"}`)}}),
		},
	}

	executor := NewEncryptedContentRetryExecutor(stub)
	stream, err := executor.DoStream(context.Background(), &httpclient.Request{
		Method:                       http.MethodPost,
		APIFormat:                    llm.APIFormatOpenAIResponse.String(),
		RetryInvalidEncryptedContent: true,
		Body:                         firstBody,
	})
	require.NoError(t, err)
	require.NotNil(t, stream)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.Len(t, stub.streamRequests, 2)
	require.Equal(t, firstBody, stub.streamRequests[0].Body)
	require.NotContains(t, string(stub.streamRequests[1].Body), "gAAAA")
}

func TestEncryptedContentRetryExecutorRetriesNonStreamingRequest(t *testing.T) {
	stub := &encryptedRetryExecutorStub{
		doErrs: []error{
			&httpclient.Error{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"code":-4201,"message":"{\"error\":{\"code\":\"invalid_encrypted_content\"}}"}`),
			},
		},
		doResponses: []*httpclient.Response{nil, {
			StatusCode: http.StatusOK,
			Body:       []byte(`{"id":"resp_1"}`),
		}},
	}

	executor := NewEncryptedContentRetryExecutor(stub)
	response, err := executor.Do(context.Background(), &httpclient.Request{
		Method:                       http.MethodPost,
		RequestType:                  llm.RequestTypeChat.String(),
		APIFormat:                    llm.APIFormatOpenAIResponse.String(),
		RetryInvalidEncryptedContent: true,
		Body:                         []byte(`{"input":[{"type":"reasoning","encrypted_content":"gAAAA"}]}`),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Len(t, stub.doRequests, 2)
	require.NotContains(t, string(stub.doRequests[1].Body), "gAAAA")
}

func TestOutboundCustomizeExecutorUsesRetryForHTTPResponses(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.test/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	stub := &encryptedRetryExecutorStub{}
	customized := outbound.CustomizeExecutor(stub)
	require.IsType(t, &encryptedContentRetryExecutor{}, customized)
	require.Same(t, customized, outbound.CustomizeExecutor(customized))
}

type encryptedRetryExecutorStub struct {
	mu             sync.Mutex
	doRequests     []*httpclient.Request
	doErrs         []error
	doResponses    []*httpclient.Response
	streamRequests []*httpclient.Request
	streamErrs     []error
	streamResults  []streams.Stream[*httpclient.StreamEvent]
}

var _ pipeline.Executor = (*encryptedRetryExecutorStub)(nil)

func (s *encryptedRetryExecutorStub) Do(_ context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doRequests = append(s.doRequests, cloneEncryptedRetryRequest(request))
	index := len(s.doRequests) - 1
	if index < len(s.doErrs) && s.doErrs[index] != nil {
		return nil, s.doErrs[index]
	}
	if index < len(s.doResponses) {
		response := s.doResponses[index]
		if response != nil {
			response.Request = request
		}
		return response, nil
	}
	return nil, errors.New("encrypted retry stub exhausted")
}

func (s *encryptedRetryExecutorStub) DoStream(_ context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamRequests = append(s.streamRequests, cloneEncryptedRetryRequest(request))
	index := len(s.streamRequests) - 1
	if index < len(s.streamErrs) && s.streamErrs[index] != nil {
		return nil, s.streamErrs[index]
	}
	if index < len(s.streamResults) {
		return s.streamResults[index], nil
	}
	return nil, errors.New("encrypted retry stream stub exhausted")
}

func cloneEncryptedRetryRequest(request *httpclient.Request) *httpclient.Request {
	if request == nil {
		return nil
	}
	cloned := *request
	cloned.Body = append([]byte(nil), request.Body...)
	cloned.JSONBody = append([]byte(nil), request.JSONBody...)
	return &cloned
}
