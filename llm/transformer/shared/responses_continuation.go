package shared

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

var ErrResponsesContinuationUnavailable = errors.New("Responses continuation unavailable")

func IsResponsesRecoveryError(err error) bool {
	if errors.Is(err, ErrResponsesContinuationUnavailable) {
		return true
	}
	var response *llm.ResponseError
	if errors.As(err, &response) {
		return response.Detail.Code == "previous_response_not_found"
	}
	var wire *httpclient.Error
	if errors.As(err, &wire) {
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		return json.Unmarshal(wire.Body, &body) == nil && body.Error.Code == "previous_response_not_found"
	}
	return false
}

func ResponsesRecoveryError() error {
	return errors.Join(ErrResponsesContinuationUnavailable, &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Status:     http.StatusText(http.StatusBadRequest),
		Body:       []byte(`{"error":{"type":"invalid_request_error","code":"previous_response_not_found","message":"Continuation context is unavailable; resend the full input without previous_response_id."}}`),
	})
}

type ResponsesOrigin struct {
	ChannelID  int
	Endpoint   string
	Format     string
	Model      string
	Auth       string
	Generation uint64
}

func (o ResponsesOrigin) SameEndpoint(other ResponsesOrigin) bool {
	o.Generation, other.Generation = 0, 0
	return o == other
}

type ResponsesAttempt struct {
	mu     sync.Mutex
	origin ResponsesOrigin
}

func (a *ResponsesAttempt) Origin() ResponsesOrigin {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.origin
}

func (a *ResponsesAttempt) SetOrigin(origin ResponsesOrigin) {
	a.mu.Lock()
	a.origin = origin
	a.mu.Unlock()
}

type responsesAttemptKey struct{}
type responsesLaneKey struct{}

func WithResponsesAttempt(ctx context.Context, attempt *ResponsesAttempt) context.Context {
	return context.WithValue(ctx, responsesAttemptKey{}, attempt)
}

func GetResponsesAttempt(ctx context.Context) *ResponsesAttempt {
	attempt, _ := ctx.Value(responsesAttemptKey{}).(*ResponsesAttempt)
	return attempt
}

func WithResponsesLane(ctx context.Context, lane *ResponsesLane) context.Context {
	return WithSessionID(context.WithValue(ctx, responsesLaneKey{}, lane), lane.ID())
}

func GetResponsesLane(ctx context.Context) *ResponsesLane {
	lane, _ := ctx.Value(responsesLaneKey{}).(*ResponsesLane)
	return lane
}
