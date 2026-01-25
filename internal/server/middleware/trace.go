package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/tracing"
)

// traceHeaderName returns the name of the header used for trace IDs.
func traceHeaderName(config tracing.Config) string {
	if config.TraceHeader != "" {
		return config.TraceHeader
	}

	return "AH-Trace-Id"
}

// getTraceIDFromHeader extracts the trace ID from the request headers.
func getTraceIDFromHeader(c *gin.Context, config tracing.Config) string {
	traceID := c.GetHeader(traceHeaderName(config))
	if traceID != "" {
		return traceID
	}

	for _, header := range config.ExtraTraceHeaders {
		traceID = c.GetHeader(header)
		if traceID != "" {
			return traceID
		}
	}

	return ""
}

// tryGetTraceIDFromBody attempts to extract a trace ID from the request body
// based on the configured ExtraTraceBodyFields.
func tryGetTraceIDFromBody(c *gin.Context, config tracing.Config) (string, error) {
	if len(config.ExtraTraceBodyFields) == 0 {
		return "", nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read request body: %w", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return "", nil
	}

	for _, field := range config.ExtraTraceBodyFields {
		result := gjson.GetBytes(body, field)
		if result.Exists() && result.String() != "" {
			return result.String(), nil
		}
	}

	return "", nil
}

// WithTrace is a middleware that extracts the X-Trace-ID header and
// gets or creates the corresponding trace entity in the database.
func WithTrace(config tracing.Config, traceService *biz.TraceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := getTraceIDFromHeader(c, config)
		if traceID == "" && config.ClaudeCodeTraceEnabled {
			var err error

			traceID, err = tryExtractTraceIDFromClaudeCodeRequest(c, config)
			if err != nil {
				AbortWithError(c, http.StatusBadRequest, err)
				return
			}
		}

		if traceID == "" && config.CodexTraceEnabled {
			traceID = tryExtractTraceIDFromCodexRequest(c)
		}

		if traceID == "" && len(config.ExtraTraceBodyFields) > 0 {
			var err error

			traceID, err = tryGetTraceIDFromBody(c, config)
			if err != nil {
				AbortWithError(c, http.StatusBadRequest, err)
				return
			}
		}

		if traceID == "" {
			c.Next()
			return
		}

		// Get project ID from context
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		// Get thread ID from context if available
		var threadID *int
		if thread, ok := contexts.GetThread(c.Request.Context()); ok && thread != nil {
			threadID = &thread.ID
		}

		// Get or create trace (errors are logged but don't block the request)
		trace, err := traceService.GetOrCreateTrace(c.Request.Context(), projectID, traceID, threadID)
		if err != nil {
			log.Warn(c.Request.Context(), "Failed to get or create trace", log.Cause(err))
			c.Next()

			return
		}

		// Store trace in context
		if log.DebugEnabled(c.Request.Context()) {
			log.Debug(c.Request.Context(), "Trace created", log.Any("trace", trace))
		}

		ctx := contexts.WithTrace(c.Request.Context(), trace)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// tryExtractTraceIDFromClaudeCodeRequest attempts to extract a trace ID from a Claude Code request.
// It checks if the request is a POST to the Anthropic messages endpoint and extracts
// the trace ID from the metadata.user_id field in the request body.
func tryExtractTraceIDFromClaudeCodeRequest(c *gin.Context, config tracing.Config) (string, error) {
	if c.Request.Method != http.MethodPost {
		return "", nil
	}

	path := c.Request.URL.Path
	if path != "/anthropic/v1/messages" && path != "/v1/messages" {
		return "", nil
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read request body: %w", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	if len(bodyBytes) == 0 {
		return "", nil
	}

	userID := gjson.GetBytes(bodyBytes, "metadata.user_id").String()
	if userID == "" {
		return "", nil
	}

	traceID := extractClaudeTraceID(userID)
	if traceID == "" {
		return "", nil
	}

	log.Debug(c.Request.Context(), "Extracted trace ID from claude code payload", log.String("trace_id", traceID))

	return traceID, nil
}

var claudeUserIDPattern = regexp.MustCompile(`(?i)^user_[0-9a-f]{64}_account__session_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const codexTraceHeader = "Session_id"

// extractClaudeTraceID parses the Claude Code user ID format to extract the session ID
// which is used as the trace ID.
func extractClaudeTraceID(userID string) string {
	if !claudeUserIDPattern.MatchString(userID) {
		return ""
	}

	traceID := userID
	if idx := strings.LastIndex(traceID, "__"); idx >= 0 && idx+2 < len(traceID) {
		traceID = traceID[idx+2:]
	}

	if idx := strings.LastIndex(traceID, "_"); idx >= 0 && idx+1 < len(traceID) {
		traceID = traceID[idx+1:]
	}

	return strings.TrimSpace(traceID)
}

// tryExtractTraceIDFromCodexRequest extracts the trace ID from the Codex session header.
func tryExtractTraceIDFromCodexRequest(c *gin.Context) string {
	traceID := strings.TrimSpace(c.GetHeader(codexTraceHeader))
	if traceID == "" {
		return ""
	}

	log.Debug(c.Request.Context(), "Extracted trace ID from codex header", log.String("trace_id", traceID))

	return traceID
}
