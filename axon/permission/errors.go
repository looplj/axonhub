package permission

import "errors"

var (
	// ErrToolCallBlocked is returned when the tool call is hard denied by the hard rules.
	ErrToolCallBlocked = errors.New("tool call blocked")

	// ErrToolCallDenied is returned when the tool call is denied by the user or the policy.
	ErrToolCallDenied = errors.New("tool call denied")
)
