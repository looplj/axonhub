package antigravity

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

// Executor implements pipeline.Executor with endpoint fallback logic for Antigravity.
// It attempts requests across multiple endpoints (Daily, Autopush, Prod) to maximize
// quota usage and reliability.
type Executor struct {
	inner         pipeline.Executor         // The underlying executor (usually httpclient.HttpClient)
	modelName     string                    // The current model being requested
	healthTracker *AntigravityHealthTracker // Tracks endpoint health per model
}

// NewExecutor creates a new Antigravity executor with endpoint fallback support.
func NewExecutor(inner pipeline.Executor) *Executor {
	if inner == nil {
		inner = httpclient.NewHttpClient()
	}

	return &Executor{
		inner:         inner,
		healthTracker: NewAntigravityHealthTracker(),
	}
}

// extractModelName extracts the model name from the request metadata.
func (e *Executor) extractModelName(request *httpclient.Request) string {
	modelName := e.modelName

	if request != nil && request.Metadata != nil {
		if model, ok := request.Metadata["antigravity_model"]; ok {
			modelName = model
		}
	}

	return modelName
}

// SetModelName sets the model name for the next request.
// This is used to determine the initial endpoint preference.
func (e *Executor) SetModelName(modelName string) {
	e.modelName = modelName
}

// Do executes an HTTP request with endpoint fallback logic.
// It tries multiple endpoints in sequence based on quota preference and retryable errors.
func (e *Executor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	// Extract model name from request metadata
	modelName := e.extractModelName(request)

	// Determine which endpoints to try based on the model
	allEndpoints := e.getEndpointsInOrder(modelName)

	// Filter out endpoints in cooldown
	var (
		availableEndpoints []string
		skippedEndpoints   []string
	)

	for _, endpoint := range allEndpoints {
		if e.healthTracker.ShouldSkip(modelName, endpoint) {
			skippedEndpoints = append(skippedEndpoints, endpoint)
			log.Debug(ctx, "skipping endpoint in cooldown",
				log.String("endpoint", endpoint),
				log.String("model", modelName))
		} else {
			availableEndpoints = append(availableEndpoints, endpoint)
		}
	}

	// If all endpoints are in cooldown, fail fast
	if len(availableEndpoints) == 0 {
		log.Warn(ctx, "all antigravity endpoints in cooldown, failing fast",
			log.String("model", modelName),
			log.Strings("skipped_endpoints", skippedEndpoints))

		return nil, fmt.Errorf("all antigravity endpoints in cooldown for model %s", modelName)
	}

	var (
		lastErr  error
		lastResp *httpclient.Response
	)

	for i, endpoint := range availableEndpoints {
		// Clone the request and update the URL for this endpoint
		reqCopy := e.cloneRequestForEndpoint(request, endpoint)

		log.Debug(ctx, "attempting antigravity request",
			log.String("endpoint", endpoint),
			log.Int("attempt", i+1),
			log.Int("available_endpoints", len(availableEndpoints)),
			log.String("model", modelName),
		)

		resp, err := e.inner.Do(ctx, reqCopy)

		// If successful (2xx), return immediately
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			e.healthTracker.RecordSuccess(modelName, endpoint)

			if i > 0 {
				log.Info(ctx, "antigravity request succeeded with fallback endpoint",
					log.String("endpoint", endpoint),
					log.Int("attempt", i+1),
				)
			}

			return resp, nil
		}

		// Store the error/response for potential return
		lastErr = err
		lastResp = resp

		// Check if we should try the next endpoint
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		} else if err != nil {
			var httpErr *httpclient.Error
			if errors.As(err, &httpErr) {
				statusCode = httpErr.StatusCode
			}
		}

		if statusCode > 0 && ShouldRetryWithDifferentEndpoint(statusCode) {
			e.healthTracker.RecordFailure(modelName, endpoint, statusCode)
			log.Warn(ctx, "antigravity request failed, trying next endpoint",
				log.String("current_endpoint", endpoint),
				log.Int("status_code", statusCode),
				log.Int("attempt", i+1),
			)

			continue
		}

		// Non-retryable error or network error - stop trying
		if err != nil {
			log.Warn(ctx, "antigravity request failed with network error",
				log.String("endpoint", endpoint),
				log.Cause(err),
			)
		} else {
			log.Debug(ctx, "antigravity request failed with non-retryable status",
				log.String("endpoint", endpoint),
				log.Int("status_code", resp.StatusCode),
			)
		}

		// Return the error/response as-is for non-retryable cases
		return resp, err
	}

	// All available endpoints exhausted
	if lastResp != nil {
		log.Error(ctx, "all available antigravity endpoints exhausted",
			log.Int("final_status", lastResp.StatusCode),
			log.String("model", modelName),
			log.Int("tried_endpoints", len(availableEndpoints)),
			log.Int("skipped_endpoints", len(skippedEndpoints)),
		)

		return lastResp, lastErr
	}

	return nil, fmt.Errorf("all antigravity endpoints failed: %w", lastErr)
}

// DoStream executes a streaming HTTP request with endpoint fallback logic.
func (e *Executor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Extract model name from request metadata
	modelName := e.extractModelName(request)

	// Determine which endpoints to try based on the model
	allEndpoints := e.getEndpointsInOrder(modelName)

	// Filter out endpoints in cooldown
	var (
		availableEndpoints []string
		skippedEndpoints   []string
	)

	for _, endpoint := range allEndpoints {
		if e.healthTracker.ShouldSkip(modelName, endpoint) {
			skippedEndpoints = append(skippedEndpoints, endpoint)
			log.Debug(ctx, "skipping endpoint in cooldown for stream",
				log.String("endpoint", endpoint),
				log.String("model", modelName))
		} else {
			availableEndpoints = append(availableEndpoints, endpoint)
		}
	}

	// If all endpoints are in cooldown, fail fast
	if len(availableEndpoints) == 0 {
		log.Warn(ctx, "all antigravity endpoints in cooldown for stream, failing fast",
			log.String("model", modelName),
			log.Strings("skipped_endpoints", skippedEndpoints))

		return nil, fmt.Errorf("all antigravity endpoints in cooldown for model %s", modelName)
	}

	var lastErr error

	for i, endpoint := range availableEndpoints {
		reqCopy := e.cloneRequestForEndpoint(request, endpoint)

		log.Debug(ctx, "attempting antigravity stream request",
			log.String("endpoint", endpoint),
			log.Int("attempt", i+1),
			log.Int("available_endpoints", len(availableEndpoints)),
			log.String("model", modelName),
		)

		stream, err := e.inner.DoStream(ctx, reqCopy)

		// If successful, return immediately
		if err == nil {
			e.healthTracker.RecordSuccess(modelName, endpoint)

			if i > 0 {
				log.Info(ctx, "antigravity stream request succeeded with fallback endpoint",
					log.String("endpoint", endpoint),
					log.Int("attempt", i+1),
				)
			}

			return stream, nil
		}

		lastErr = err

		// Check if we should try the next endpoint for streaming
		// httpclient.DoStream returns nil stream and error for status >= 400
		statusCode := 0
		if err != nil {
			var httpErr *httpclient.Error
			if errors.As(err, &httpErr) {
				statusCode = httpErr.StatusCode
			}
		}

		// For streaming, we record failure if we have an error or bad status
		// If we have a specific status code, check if it's retryable
		// If we just have a generic error, we might want to retry (network errors)
		// but for now let's stick to status codes if available, or just record failure
		e.healthTracker.RecordFailure(modelName, endpoint, statusCode)

		if statusCode > 0 && ShouldRetryWithDifferentEndpoint(statusCode) {
			log.Warn(ctx, "antigravity stream request failed with retryable status, trying next endpoint",
				log.String("current_endpoint", endpoint),
				log.Int("status_code", statusCode),
				log.Int("attempt", i+1),
			)
			continue
		}

		// For other errors (network errors without status code), existing logic just logs and continues?
		// The original code was:
		// e.healthTracker.RecordFailure(modelName, endpoint, 0)
		// log.Warn(ctx, "antigravity stream request failed, trying next endpoint", ...)
		// It ALWAYS retried on error for streaming.
		// Let's preserve that aggressive retry for streaming unless we KNOW it's non-retryable (e.g. 400 Bad Request)

		if statusCode > 0 && !ShouldRetryWithDifferentEndpoint(statusCode) {
			// Non-retryable status code (e.g. 400), return error immediately
			return nil, err
		}

		log.Warn(ctx, "antigravity stream request failed, trying next endpoint",
			log.String("current_endpoint", endpoint),
			log.Int("attempt", i+1),
			log.Cause(err),
		)
	}

	log.Error(ctx, "all available antigravity stream endpoints exhausted",
		log.String("model", modelName),
		log.Int("tried_endpoints", len(availableEndpoints)),
		log.Int("skipped_endpoints", len(skippedEndpoints)),
		log.Cause(lastErr),
	)

	return nil, fmt.Errorf("all antigravity stream endpoints failed: %w", lastErr)
}

// getEndpointsInOrder returns the ordered list of endpoints to try based on model preference.
func (e *Executor) getEndpointsInOrder(modelName string) []string {
	if modelName == "" {
		// No model specified, use default fallback order
		return GetFallbackEndpoints()
	}

	// Determine quota preference for this model
	quotaPreference := DetermineQuotaPreference(modelName)
	initialEndpoint := GetInitialEndpoint(quotaPreference)

	// Get all fallback endpoints
	fallbacks := GetFallbackEndpoints()

	// Reorder so the preferred endpoint is first
	ordered := make([]string, 0, len(fallbacks))
	ordered = append(ordered, initialEndpoint)

	for _, ep := range fallbacks {
		if ep != initialEndpoint {
			ordered = append(ordered, ep)
		}
	}

	return ordered
}

// cloneRequestForEndpoint creates a copy of the request with the URL updated for the given endpoint.
func (e *Executor) cloneRequestForEndpoint(request *httpclient.Request, endpoint string) *httpclient.Request {
	if request == nil {
		return nil
	}

	// Create a shallow copy
	copied := *request

	if request.Headers != nil {
		copied.Headers = request.Headers.Clone()
	}

	if len(request.Query) > 0 {
		copied.Query = make(url.Values, len(request.Query))
		for k, v := range request.Query {
			if v != nil {
				copied.Query[k] = append([]string(nil), v...)
			}
		}
	}

	if len(request.Body) > 0 {
		copied.Body = append([]byte(nil), request.Body...)
	}

	if len(request.JSONBody) > 0 {
		copied.JSONBody = append([]byte(nil), request.JSONBody...)
	}

	if len(request.Metadata) > 0 {
		copied.Metadata = make(map[string]string, len(request.Metadata))
		maps.Copy(copied.Metadata, request.Metadata)
	}

	if len(request.TransformerMetadata) > 0 {
		copied.TransformerMetadata = make(map[string]any, len(request.TransformerMetadata))
		maps.Copy(copied.TransformerMetadata, request.TransformerMetadata)
	}

	// Update the URL to use the new endpoint
	// The URL format is: {endpoint}/v1internal:{action}
	// We need to replace the base URL while preserving the path
	copied.URL = replaceBaseURL(request.URL, endpoint)

	return &copied
}

// replaceBaseURL replaces the base URL portion while preserving the path.
// Example: replaceBaseURL("https://daily.../v1internal:generateContent", "https://prod...")
// Returns: "https://prod.../v1internal:generateContent"
func replaceBaseURL(originalURL, newBase string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		log.Warn(context.Background(), "failed to parse original URL in replaceBaseURL",
			log.String("originalURL", originalURL),
			log.Cause(err))

		return originalURL
	}

	if parsed.Path == "" {
		log.Warn(context.Background(), "original URL has no path",
			log.String("originalURL", originalURL))

		return originalURL
	}

	if !strings.HasPrefix(parsed.Path, "/v1internal") {
		log.Warn(context.Background(), "original URL does not contain expected /v1internal path segment",
			log.String("originalURL", originalURL),
			log.String("path", parsed.Path))

		return originalURL
	}

	newBaseParsed, err := url.Parse(newBase)
	if err != nil {
		log.Warn(context.Background(), "failed to parse newBase in replaceBaseURL",
			log.String("newBase", newBase),
			log.Cause(err))

		return originalURL
	}

	newBaseParsed.Path = parsed.Path
	newBaseParsed.RawQuery = parsed.RawQuery
	newBaseParsed.Fragment = parsed.Fragment

	return newBaseParsed.String()
}
