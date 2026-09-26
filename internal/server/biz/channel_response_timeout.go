package biz

import (
	"fmt"

	"github.com/looplj/axonhub/internal/objects"
)

// ValidateResponseTimeout checks the per-channel response timeout overrides.
// A nil settings object, or one whose mode is inherit, means the channel follows
// the global retry policy and is always valid; any stale override values are
// ignored in that mode and therefore not validated.
//
// A configured timeout must be within [0, maxRetryResponseTimeoutSeconds], the
// same bound the global retry policy enforces when it normalizes its own values.
func ValidateResponseTimeout(settings *objects.ChannelResponseTimeoutSettings) error {
	if !settings.IsCustom() {
		return nil
	}

	if err := validateResponseTimeoutSeconds("streamFirstEventTimeoutSeconds", settings.StreamFirstEventTimeoutSeconds); err != nil {
		return err
	}

	return validateResponseTimeoutSeconds("nonStreamResponseTimeoutSeconds", settings.NonStreamResponseTimeoutSeconds)
}

func validateResponseTimeoutSeconds(name string, seconds *int) error {
	if seconds == nil {
		return nil
	}

	if *seconds < 0 {
		return fmt.Errorf("%s must be >= 0", name)
	}

	if *seconds > maxRetryResponseTimeoutSeconds {
		return fmt.Errorf("%s must be <= %d", name, maxRetryResponseTimeoutSeconds)
	}

	return nil
}
