package orchestrator

import (
	"errors"

	"github.com/looplj/axonhub/llm"
)

// PluginOverloadedError is returned by an onAttempt hook when the plugin runtime
// pool cannot lend a VM. Unwrap returns the 429 the client should see.
// The pipeline must not retry the same channel or switch channels.
type PluginOverloadedError struct {
	Response *llm.ResponseError
}

// NewPluginOverloadedError wraps the client-facing 429.
func NewPluginOverloadedError(response *llm.ResponseError) *PluginOverloadedError {
	return &PluginOverloadedError{Response: response}
}

func (e *PluginOverloadedError) Error() string {
	if e == nil || e.Response == nil {
		return "plugin overloaded"
	}
	return e.Response.Error()
}

// Unwrap returns the 429 *llm.ResponseError so inbound TransformError can surface it.
func (e *PluginOverloadedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Response
}

func isPluginOverloaded(err error) bool {
	var overloaded *PluginOverloadedError
	return errors.As(err, &overloaded)
}
