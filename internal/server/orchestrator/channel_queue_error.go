package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	channelQueueReasonFull    = "queue_full"
	channelQueueReasonTimeout = "queue_timeout"
)

// ChannelQueueError represents a channel-level admission failure raised by
// ChannelLimiter inside the channelLimiter middleware.
//
// It joins the typed cause (ErrChannelQueueFull / ErrChannelQueueTimeout) with
// a synthetic *httpclient.Error so the inbound TransformError path can pluck
// out a 429 response shape via errors.AsType[*httpclient.Error].
//
// No Retry-After is set: the chat handler drops headers from synthetic errors,
// and its absence keeps HasRetryAfterHeader off so this local rejection cannot
// be misread as an upstream 429 by the cooldown middleware.
type ChannelQueueError struct {
	ChannelID   int
	ChannelName string
	Reason      string
	Cause       error

	httpErr *httpclient.Error
}

// asChannelQueueError wraps a ChannelLimiter sentinel error with channel context.
// Returns nil when err is not a queue admission error.
func asChannelQueueError(ch *biz.Channel, err error) *ChannelQueueError {
	if ch == nil || err == nil {
		return nil
	}

	var reason string
	switch {
	case errors.Is(err, ErrChannelQueueFull):
		reason = channelQueueReasonFull
	case errors.Is(err, ErrChannelQueueTimeout):
		reason = channelQueueReasonTimeout
	default:
		return nil
	}

	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":    "rate_limit_error",
			"code":    "channel_" + reason,
			"message": fmt.Sprintf("channel %s is at capacity (%s); please retry shortly", ch.Name, reason),
		},
	})

	return &ChannelQueueError{
		ChannelID:   ch.ID,
		ChannelName: ch.Name,
		Reason:      reason,
		Cause:       err,
		httpErr: &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Status:     http.StatusText(http.StatusTooManyRequests),
			Body:       body,
		},
	}
}

func (e *ChannelQueueError) Error() string {
	return fmt.Sprintf("channel %q (id=%d) %s", e.ChannelName, e.ChannelID, e.Reason)
}

// Unwrap exposes both the typed sentinel and the synthetic transport error so
// errors.Is matches ErrChannelQueueFull / ErrChannelQueueTimeout while
// errors.AsType[*httpclient.Error] finds the 429 response shape.
func (e *ChannelQueueError) Unwrap() []error {
	return []error{e.Cause, e.httpErr}
}

// isChannelQueueError reports whether err is or wraps a *ChannelQueueError.
// Used by error-tracking middlewares to skip handlers that would otherwise
// treat the synthetic 429 as an upstream signal.
func isChannelQueueError(err error) bool {
	var qe *ChannelQueueError
	return errors.As(err, &qe)
}
