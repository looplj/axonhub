package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/tracing"
)

// AccessLog returns a middleware that logs access information for each request.
// It logs: status code, method, path, graphql operation (if applicable), and errors.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		ctx := c.Request.Context()

		// Collect errors from gin context and request context
		var errMsgs []string
		for _, e := range c.Errors {
			errMsgs = append(errMsgs, e.Error())
		}

		for _, e := range contexts.GetErrors(ctx) {
			errMsgs = append(errMsgs, e.Error())
		}

		// Only log if there are errors or status >= 400
		status := c.Writer.Status()
		if status < 400 && len(errMsgs) == 0 {
			return
		}

		latency := time.Since(start)

		fields := []log.Field{
			log.Int("status", status),
			log.String("method", c.Request.Method),
			log.String("path", c.Request.URL.Path),
			log.Duration("latency", latency),
			log.String("client_ip", c.ClientIP()),
		}

		// Add GraphQL operation name if available
		if opName, ok := tracing.GetOperationName(ctx); ok {
			fields = append(fields, log.String("operation", opName))
		}

		// Add errors if present
		if len(errMsgs) > 0 {
			fields = append(fields, log.Strings("errors", errMsgs))
		}

		log.Error(ctx, "[ACCESS]", fields...)
	}
}
