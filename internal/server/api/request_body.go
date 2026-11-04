package api

import (
	"errors"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// readHTTPRequest 在 HTTP 入口将原始体和解压后超限统一为 413，交给各协议格式化错误。
// 这类错误发生在上游调用之前，不应被当作服务器故障触发客户端重试。
func readHTTPRequest(request *http.Request) (*httpclient.Request, error) {
	parsed, err := httpclient.ReadHTTPRequest(request)
	if errors.Is(err, httpclient.ErrRequestBodyTooLarge) {
		return nil, &llm.ResponseError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Detail: llm.ErrorDetail{
				Type:    "request_too_large",
				Code:    "request_body_too_large",
				Message: err.Error(),
			},
		}
	}
	return parsed, err
}
