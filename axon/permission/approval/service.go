package approval

import (
	"context"
	"encoding/json"

	"github.com/looplj/axonhub/axon/permission/grant"
)

type Request struct {
	ID         string
	ThreadID   string
	Workspace  string
	ToolCallID string
	ToolName   string

	Summary   string
	Reason    string
	RiskLevel string

	// JSON-safe, redacted resources for UI display.
	Resources json.RawMessage
}

type Response struct {
	Granted bool
	Scope   grant.Scope
	Reason  string
}

// Service manages approval requests that require a UI subscriber to grant or deny.
// It supports publishing requests to subscribers and waiting for a resolution.
type Service interface {
	// Subscribe registers a subscriber (typically UI) to receive approval requests.
	// The returned channel will be closed when ctx is done.
	Subscribe(ctx context.Context) <-chan Request

	// Request publishes an approval request to subscribers and blocks until it is granted/denied,
	// or until ctx is done. It returns an error if req.ID is empty or if no subscribers exist.
	Request(ctx context.Context, req Request) (Response, error)

	// Grant resolves a pending approval request with the given scope.
	Grant(req Request, scope grant.Scope) error

	// Deny resolves a pending approval request as denied.
	Deny(req Request) error

	// Active returns the currently active request, if any.
	Active() (Request, bool)
}
