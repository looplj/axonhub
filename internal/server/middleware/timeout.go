package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/llm/httpclient"
)

func WithTimeout(ts time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), ts)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

type llmTimeoutKey struct{}

type llmTimeoutPolicy struct {
	started         time.Time
	request, stream time.Duration
}

// WithLLMTimeout keeps the larger budget until the parsed request identifies
// its response mode. A zero budget must remain unlimited during that phase.
func WithLLMTimeout(requestTimeout, streamTimeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		policy := llmTimeoutPolicy{started: time.Now(), request: requestTimeout, stream: streamTimeout}
		timeout := requestTimeout
		if c.Request.Method == http.MethodPost {
			timeout = max(requestTimeout, streamTimeout)
			if requestTimeout == 0 || streamTimeout == 0 {
				timeout = 0
			}
		}
		ctx := context.WithValue(c.Request.Context(), llmTimeoutKey{}, policy)
		var cancel context.CancelFunc
		if timeout == 0 {
			ctx, cancel = context.WithCancel(ctx)
		} else {
			ctx, cancel = context.WithDeadline(ctx, policy.started.Add(timeout))
		}
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		if timeout == 0 {
			writer := &llmResponseWriter{ResponseWriter: c.Writer, timeout: llmWriteTimeout}
			c.Writer = writer
			writer.setDeadline(time.Time{})
			// net/http may flush buffered headers/body after Gin returns.
			defer func() { writer.setDeadline(time.Now().Add(writer.timeout)) }()
		}
		c.Next()
	}
}

// SelectLLMTimeout applies the response-mode budget from the original arrival
// time, including time spent reading and preparing the request.
func SelectLLMTimeout(c *gin.Context, streaming bool) context.CancelFunc {
	policy, ok := c.Request.Context().Value(llmTimeoutKey{}).(llmTimeoutPolicy)
	if !ok {
		return func() {}
	}
	timeout := policy.request
	if streaming {
		timeout = policy.stream
	}
	if timeout == 0 {
		return func() {}
	}
	ctx, cancel := context.WithDeadline(c.Request.Context(), policy.started.Add(timeout))
	c.Request = c.Request.WithContext(ctx)
	return cancel
}

// WithStreamIdleTimeout configures upstream reads without counting downstream keep-alives.
func WithStreamIdleTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := httpclient.WithStreamIdleTimeout(c.Request.Context(), timeout)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
