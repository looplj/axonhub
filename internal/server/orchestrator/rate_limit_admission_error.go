package orchestrator

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

var ErrLocalRPMExhausted = errors.New("local channel rpm exhausted")

// LocalRPMExhaustedError represents a local per-channel RPM admission failure.
//
// The embedded synthetic 429 lets existing response transformation produce a
// rate-limit-shaped client error, while the typed wrapper lets retry and
// cooldown code distinguish it from provider 429 responses.
type LocalRPMExhaustedError struct {
	ChannelID   int
	ChannelName string
	Limit       int64

	httpErr *httpclient.Error
}

func newLocalRPMExhaustedError(ch *biz.Channel, limit int64) *LocalRPMExhaustedError {
	message := fmt.Sprintf("channel %s exhausted local RPM limit %d; please retry shortly", ch.Name, limit)
	body := fmt.Appendf(nil, `{"error":{"code":"channel_rpm_exhausted","message":%q,"type":"rate_limit_error"}}`, message)

	return &LocalRPMExhaustedError{
		ChannelID:   ch.ID,
		ChannelName: ch.Name,
		Limit:       limit,
		httpErr: &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Status:     http.StatusText(http.StatusTooManyRequests),
			Body:       body,
		},
	}
}

func (e *LocalRPMExhaustedError) Error() string {
	return fmt.Sprintf("channel %q (id=%d) local rpm exhausted", e.ChannelName, e.ChannelID)
}

func (e *LocalRPMExhaustedError) Unwrap() []error {
	return []error{ErrLocalRPMExhausted, e.httpErr}
}

func isLocalRPMExhaustedError(err error) bool {
	var rpmErr *LocalRPMExhaustedError
	return errors.As(err, &rpmErr)
}
