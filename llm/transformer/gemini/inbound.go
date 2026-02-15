package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/transformer"
)

var ErrInvalidRequestURL = errors.New("invalid request URL")

// InboundTransformer implements transformer.Inbound for Gemini format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new Gemini InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

// APIFormat returns the API format of the transformer.
func (t *InboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatGeminiContents
}

// extractRequestParams extracts the model and stream flag from the request URL.
func extractRequestParams(httpReq *httpclient.Request) (string, bool, error) {
	urlParts := strings.Split(httpReq.Path, "/")
	if len(urlParts) < 1 {
		return "", false, fmt.Errorf("%w: invalid request path: %s", ErrInvalidRequestURL, httpReq.Path)
	}

	suffix := urlParts[len(urlParts)-1]

	suffixParts := strings.Split(suffix, ":")
	if len(suffixParts) < 2 {
		return "", false, fmt.Errorf("%w: invalid request path: %s", ErrInvalidRequestURL, httpReq.Path)
	}

	switch suffixParts[1] {
	case "generateContent":
		return suffixParts[0], false, nil
	case "streamGenerateContent":
		return suffixParts[0], true, nil
	default:
		return "", false, fmt.Errorf("%w: invalid request path: %s", ErrInvalidRequestURL, httpReq.Path)
	}
}

// TransformRequest transforms Gemini HTTP request to unified Request format.
func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	model, stream, err := extractRequestParams(httpReq)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "extract gemini request params", slog.String("model", model), slog.Bool("stream", stream))

	var geminiReq GenerateContentRequest
	if err := json.Unmarshal(httpReq.Body, &geminiReq); err != nil {
		return nil, fmt.Errorf("%w: failed to decode gemini request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if len(geminiReq.Contents) == 0 {
		return nil, fmt.Errorf("%w: contents are required", transformer.ErrInvalidRequest)
	}

	req, err := convertGeminiToLLMRequest(&geminiReq)
	if err != nil {
		return nil, err
	}

	req.Stream = &stream
	req.Model = model

	return req, nil
}

// TransformResponse transforms the unified response format to Gemini HTTP response.
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *llm.Response) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to Gemini response format (non-streaming)
	geminiResp := convertLLMToGeminiResponse(chatResp, false)

	body, err := json.Marshal(geminiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

// TransformError transforms the unified error response to HTTP error response in Gemini format.
func (t *InboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body: xjson.MustMarshal(&GeminiError{
				Error: ErrorDetail{
					Code:    http.StatusInternalServerError,
					Message: http.StatusText(http.StatusInternalServerError),
					Status:  mapHTTPStatusToGeminiStatus(http.StatusInternalServerError),
				},
			}),
		}
	}

	if errors.Is(rawErr, ErrInvalidRequestURL) {
		return &httpclient.Error{
			StatusCode: http.StatusNotFound,
			Status:     http.StatusText(http.StatusNotFound),
			Body: xjson.MustMarshal(&GeminiError{
				Error: ErrorDetail{
					Code:    http.StatusNotFound,
					Message: rawErr.Error(),
					Status:  mapHTTPStatusToGeminiStatus(http.StatusNotFound),
				},
			}),
		}
	}

	llmErr := &llm.ResponseError{}
	if errors.As(rawErr, &llmErr) {
		return &httpclient.Error{
			StatusCode: llmErr.StatusCode,
			Status:     http.StatusText(llmErr.StatusCode),
			Body: xjson.MustMarshal(&GeminiError{
				Error: ErrorDetail{
					Code:    llmErr.StatusCode,
					Message: llmErr.Detail.Message,
					Status:  mapHTTPStatusToGeminiStatus(llmErr.StatusCode),
				},
			}),
		}
	}

	httpErr := &httpclient.Error{}
	if errors.As(rawErr, &httpErr) {
		return httpErr
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) || errors.Is(rawErr, transformer.ErrInvalidModel) {
		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body: xjson.MustMarshal(&GeminiError{
				Error: ErrorDetail{
					Code:    http.StatusBadRequest,
					Message: rawErr.Error(),
					Status:  mapHTTPStatusToGeminiStatus(http.StatusBadRequest),
				},
			}),
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body: xjson.MustMarshal(&GeminiError{
			Error: ErrorDetail{
				Code:    http.StatusInternalServerError,
				Message: http.StatusText(http.StatusInternalServerError),
				Status:  mapHTTPStatusToGeminiStatus(http.StatusInternalServerError),
			},
		}),
	}
}

// mapHTTPStatusToGeminiStatus maps HTTP status codes to Gemini status strings.
func mapHTTPStatusToGeminiStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "INVALID_ARGUMENT"
	case http.StatusUnauthorized:
		return "UNAUTHENTICATED"
	case http.StatusForbidden:
		return "PERMISSION_DENIED"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "ALREADY_EXISTS"
	case http.StatusTooManyRequests:
		return "RESOURCE_EXHAUSTED"
	case http.StatusInternalServerError:
		return "INTERNAL"
	case http.StatusNotImplemented:
		return "UNIMPLEMENTED"
	case http.StatusServiceUnavailable:
		return "UNAVAILABLE"
	default:
		return "UNKNOWN"
	}
}
