package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func transformOrchestratorError(ctx context.Context, err error, orch *orchestrator.ChatCompletionOrchestrator) *httpclient.Error {
	err = wrapQuotaExhaustedAsResponseError(err)
	if orch != nil {
		err = applyUpstreamErrorPolicy(ctx, err, orch.SystemService)
		return orch.Inbound.TransformError(ctx, err)
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       []byte(`{"error":{"message":"internal server error","type":"internal_server_error"}}`),
	}
}

func applyUpstreamErrorPolicy(ctx context.Context, err error, systemService *biz.SystemService) error {
	if err == nil || systemService == nil {
		return err
	}

	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return err
	}

	policy := systemService.RetryPolicyOrDefault(ctx).UpstreamErrorPolicy
	if policy.Mode == biz.UpstreamErrorModePassthrough || policy.Mode == "" {
		return err
	}

	if !pipeline.IsUpstreamError(err) {
		return err
	}

	message := biz.DefaultUpstreamErrorMessage
	if policy.Mode == biz.UpstreamErrorModeCustom {
		message = strings.TrimSpace(policy.CustomMessage)
		if message == "" {
			message = biz.DefaultUpstreamErrorMessage
		}
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) {
		if respErr.Detail.Code == errCodeQuotaExhausted || respErr.Detail.Type == errTypeQuotaExhausted {
			return err
		}

		return &llm.ResponseError{
			StatusCode: respErr.StatusCode,
			Detail: llm.ErrorDetail{
				Message:   message,
				Type:      firstNonEmpty(respErr.Detail.Type, "upstream_error"),
				Code:      respErr.Detail.Code,
				RequestID: respErr.Detail.RequestID,
			},
		}
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		return &llm.ResponseError{
			StatusCode: httpErr.StatusCode,
			Detail: llm.ErrorDetail{
				Message:   message,
				Type:      upstreamErrorTypeFromHTTP(httpErr),
				Code:      upstreamErrorCodeFromHTTP(httpErr),
				RequestID: upstreamRequestIDFromHTTP(httpErr),
			},
		}
	}

	return &llm.ResponseError{
		StatusCode: http.StatusBadGateway,
		Detail: llm.ErrorDetail{
			Message: message,
			Type:    "upstream_error",
		},
	}
}

type upstreamErrorStream struct {
	stream        streams.Stream[*httpclient.StreamEvent]
	ctx           context.Context
	systemService *biz.SystemService
}

func newUpstreamErrorStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent], systemService *biz.SystemService) streams.Stream[*httpclient.StreamEvent] {
	if stream == nil || systemService == nil {
		return stream
	}

	return &upstreamErrorStream{
		stream:        stream,
		ctx:           ctx,
		systemService: systemService,
	}
}

func (s *upstreamErrorStream) Next() bool {
	return s.stream.Next()
}

func (s *upstreamErrorStream) Current() *httpclient.StreamEvent {
	return s.stream.Current()
}

func (s *upstreamErrorStream) Err() error {
	err := s.stream.Err()
	if err == nil {
		return nil
	}

	policy := s.systemService.RetryPolicyOrDefault(s.ctx).UpstreamErrorPolicy
	if policy.Mode != "" && policy.Mode != biz.UpstreamErrorModePassthrough {
		return applyUpstreamErrorPolicy(s.ctx, pipeline.WrapUpstreamError(err), s.systemService)
	}

	return err
}

func (s *upstreamErrorStream) Close() error {
	return s.stream.Close()
}

func upstreamErrorTypeFromHTTP(httpErr *httpclient.Error) string {
	if httpErr != nil && len(httpErr.Body) > 0 {
		if t := gjson.GetBytes(httpErr.Body, "error.type"); t.Exists() && t.Type == gjson.String && t.String() != "" {
			return t.String()
		}
	}

	return "upstream_error"
}

func upstreamErrorCodeFromHTTP(httpErr *httpclient.Error) string {
	if httpErr != nil && len(httpErr.Body) > 0 {
		if c := gjson.GetBytes(httpErr.Body, "error.code"); c.Exists() && c.Type == gjson.String && c.String() != "" {
			return c.String()
		}
	}

	return ""
}

func upstreamRequestIDFromHTTP(httpErr *httpclient.Error) string {
	if httpErr != nil && len(httpErr.Body) > 0 {
		if rid := gjson.GetBytes(httpErr.Body, "request_id"); rid.Exists() && rid.Type == gjson.String && rid.String() != "" {
			return rid.String()
		}
	}

	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
