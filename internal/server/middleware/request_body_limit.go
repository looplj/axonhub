package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/llm/httpclient"
)

// WithRequestBodyLimit 在追踪中间件之前限制原始 Body，避免 trace 提取先进行无界读取。
// 上限来自启动时的环境变量配置；管理端设置和备份恢复接口不受此限制。
func WithRequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		maxBytes := httpclient.RequestBodyLimit(c.Request.Context())
		if c.Request.ContentLength > maxBytes {
			// 超限属于客户端请求错误，返回 413 避免客户端按服务器故障重试。
			AbortWithError(c, http.StatusRequestEntityTooLarge, fmt.Errorf("failed to read request body: %w", httpclient.ErrRequestBodyTooLarge))
			return
		}
		if c.Request.Body != nil && c.Request.Body != http.NoBody {
			// 未知长度和 chunked 请求也按实际读取字节限制；nil writer 不触发隐式状态码处理。
			c.Request.Body = http.MaxBytesReader(nil, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// abortTraceBodyReadError 让追踪阶段提前读到的请求体超限返回 HTTP 413，其余解析错误仍为 400。
func abortTraceBodyReadError(c *gin.Context, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) || errors.Is(err, httpclient.ErrRequestBodyTooLarge) {
		AbortWithError(c, http.StatusRequestEntityTooLarge, fmt.Errorf("failed to read request body: %w", httpclient.ErrRequestBodyTooLarge))
		return
	}
	AbortWithError(c, http.StatusBadRequest, err)
}
