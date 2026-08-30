package responses

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

const invalidEncryptedContentCode = "invalid_encrypted_content"

// encryptedContentRetryExecutor retries a Responses request once after an
// upstream reports that an account-bound encrypted item cannot be verified.
// The retry uses the same underlying executor and only removes opaque encrypted
// fields from the request body.
type encryptedContentRetryExecutor struct {
	inner pipeline.Executor
}

var _ pipeline.Executor = (*encryptedContentRetryExecutor)(nil)

// NewEncryptedContentRetryExecutor decorates an executor with the one-shot
// invalid_encrypted_content recovery used by Responses API channels.
func NewEncryptedContentRetryExecutor(inner pipeline.Executor) pipeline.Executor {
	if inner == nil {
		return nil
	}

	if existing, ok := inner.(*encryptedContentRetryExecutor); ok {
		return existing
	}

	return &encryptedContentRetryExecutor{inner: inner}
}

func (e *encryptedContentRetryExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	if e == nil || e.inner == nil {
		return nil, errors.New("encrypted content retry executor is not initialized")
	}

	response, err := e.inner.Do(ctx, request)
	if retryRequest, ok := PrepareEncryptedContentRetryRequest(request, response, err); ok {
		return e.inner.Do(ctx, retryRequest)
	}

	return response, err
}

func (e *encryptedContentRetryExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	if e == nil || e.inner == nil {
		return nil, errors.New("encrypted content retry executor is not initialized")
	}

	stream, err := e.inner.DoStream(ctx, request)
	if retryRequest, ok := PrepareEncryptedContentRetryRequest(request, nil, err); ok {
		if stream != nil {
			_ = stream.Close()
		}

		return e.inner.DoStream(ctx, retryRequest)
	}

	return stream, err
}

// PrepareEncryptedContentRetryRequest returns a request copy with opaque,
// account-bound encrypted content removed when response/err identifies a 400
// invalid_encrypted_content failure. Callers should issue at most one retry
// with the returned request.
func PrepareEncryptedContentRetryRequest(
	request *httpclient.Request,
	response *httpclient.Response,
	err error,
) (*httpclient.Request, bool) {
	if request == nil || !isResponsesRequest(request) {
		return nil, false
	}

	if responseStatusCode(response, err) != http.StatusBadRequest || !isInvalidEncryptedContentError(err, response) {
		return nil, false
	}

	strippedBody, stripped := stripEncryptedReasoningContent(request.Body)
	if !stripped {
		return nil, false
	}

	retryRequest := *request
	retryRequest.Body = strippedBody
	if len(request.JSONBody) > 0 {
		retryRequest.JSONBody = append([]byte(nil), strippedBody...)
	}

	return &retryRequest, true
}

func isResponsesRequest(request *httpclient.Request) bool {
	if request == nil {
		return false
	}

	switch request.RequestType {
	case llm.RequestTypeImage.String(), llm.RequestTypeAlphaSearch.String():
		return false
	}

	switch request.APIFormat {
	case llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseCompact.String():
		return true
	case "":
		// Keep direct executor users and tests working when APIFormat was not
		// populated, while still requiring the Responses input shape.
		return gjson.GetBytes(request.Body, "input").Exists()
	default:
		return false
	}
}

func responseStatusCode(response *httpclient.Response, err error) int {
	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && httpErr != nil && httpErr.StatusCode != 0 {
		return httpErr.StatusCode
	}

	if response != nil {
		return response.StatusCode
	}

	return 0
}

func responseErrorBody(response *httpclient.Response, err error) []byte {
	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && httpErr != nil && len(httpErr.Body) > 0 {
		return httpErr.Body
	}

	if response != nil && len(response.Body) > 0 {
		return response.Body
	}

	return nil
}

// isInvalidEncryptedContentError accepts the regular OpenAI error envelope
// and gateway envelopes that put a JSON error object inside message.
func isInvalidEncryptedContentError(err error, response *httpclient.Response) bool {
	body := responseErrorBody(response, err)
	if len(bytes.TrimSpace(body)) == 0 {
		return false
	}

	// Walk the small number of envelopes used by Responses gateways. ModelHub
	// commonly returns {"code":-4201,"message":"<JSON error>"}.
	originalBody := body
	hasGatewayCode := false
	for depth := 0; depth < 4; depth++ {
		if gjson.GetBytes(body, "code").Int() == -4201 {
			hasGatewayCode = true
		}

		for _, path := range []string{"error.code", "message.error.code", "code"} {
			if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(body, path).String()), invalidEncryptedContentCode) {
				return true
			}
		}

		message := gjson.GetBytes(body, "message")
		if message.Type != gjson.String {
			break
		}
		nested := strings.TrimSpace(message.String())
		if nested == "" || nested == string(body) {
			break
		}
		body = []byte(nested)
	}

	// A few gateways preserve only their numeric error code at the outer layer
	// and leave the provider code in plain text. Keep this fallback narrow.
	return hasGatewayCode && strings.Contains(string(originalBody), invalidEncryptedContentCode)
}

// stripEncryptedReasoningContent removes account-bound encrypted blobs while
// retaining visible messages and reasoning summaries. The supported wire
// shapes are a reasoning item's encrypted_content field and encrypted_content
// variants nested in function_call_output.output.
func stripEncryptedReasoningContent(body []byte) ([]byte, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false
	}

	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, false
	}

	out := body
	stripped := false

	for i, item := range input.Array() {
		if strings.EqualFold(item.Get("type").String(), "reasoning") {
			encrypted := item.Get("encrypted_content")
			if encrypted.Exists() && encrypted.Type != gjson.Null {
				if next, err := sjson.DeleteBytes(out, fmt.Sprintf("input.%d.encrypted_content", i)); err == nil {
					out = next
					stripped = true
				}
			}
		}

		output := item.Get("output")
		if !output.IsArray() {
			continue
		}

		// Delete array elements in reverse order so indexes remain stable.
		outputItems := output.Array()
		for j := len(outputItems) - 1; j >= 0; j-- {
			if !strings.EqualFold(outputItems[j].Get("type").String(), "encrypted_content") {
				continue
			}
			if next, err := sjson.DeleteBytes(out, fmt.Sprintf("input.%d.output.%d", i, j)); err == nil {
				out = next
				stripped = true
			}
		}
	}

	return out, stripped
}
