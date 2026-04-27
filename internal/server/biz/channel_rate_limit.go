package biz

import (
	"fmt"

	"github.com/looplj/axonhub/internal/objects"
)

// ValidateRateLimit checks ChannelRateLimit invariants enforced at the API boundary:
//   - all numeric fields must be >= 0 when set
//   - QueueSize > 0 requires MaxConcurrent > 0 (a queue without a capacity ceiling
//     has no meaning)
//
// Returning a nil rate limit is valid (no admission control configured).
func ValidateRateLimit(rl *objects.ChannelRateLimit) error {
	if rl == nil {
		return nil
	}

	if err := nonNegativeRateLimitField("rpm", rl.RPM); err != nil {
		return err
	}

	if err := nonNegativeRateLimitField("tpm", rl.TPM); err != nil {
		return err
	}

	if err := nonNegativeRateLimitField("maxConcurrent", rl.MaxConcurrent); err != nil {
		return err
	}

	if err := nonNegativeRateLimitField("queueSize", rl.QueueSize); err != nil {
		return err
	}

	if err := nonNegativeRateLimitField("queueTimeoutMs", rl.QueueTimeoutMs); err != nil {
		return err
	}

	if rl.QueueSize != nil && *rl.QueueSize > 0 {
		if rl.MaxConcurrent == nil || *rl.MaxConcurrent <= 0 {
			return fmt.Errorf("queueSize requires maxConcurrent > 0")
		}
	}

	return nil
}

func nonNegativeRateLimitField(name string, v *int64) error {
	if v != nil && *v < 0 {
		return fmt.Errorf("%s must be >= 0", name)
	}

	return nil
}
