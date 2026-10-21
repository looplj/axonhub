package provider_quota

import (
	"strings"
)

// MatchLimitModel checks whether a limit applies to the requested model.
// When a limit has no AvailabilityGroup or is not an Antigravity pool limit,
// it applies to all models.
func MatchLimitModel(limit QuotaLimitStatus, actualModel string) bool {
	if limit.AvailabilityGroup == "" || actualModel == "" {
		return true
	}
	if !strings.HasPrefix(limit.AvailabilityGroup, antigravityPoolAvailabilityGroupPrefix) {
		return true
	}
	pool := strings.TrimPrefix(limit.AvailabilityGroup, antigravityPoolAvailabilityGroupPrefix)
	modelPool := antigravityModelPool(actualModel)
	if modelPool == "" {
		return true
	}
	return pool == modelPool
}

// FilterLimitsForModel keeps only the limits that match the requested actual model.
// For Antigravity pool limits, stripping limits of other pools also clears the
// pool-specific AvailabilityGroup so that an exhausted window in the targeted pool
// triggers exhaustion instead of being OR'd away as an alternative capacity group.
func FilterLimitsForModel(limits []QuotaLimitStatus, actualModel string) []QuotaLimitStatus {
	if actualModel == "" || len(limits) == 0 {
		return limits
	}
	filtered := make([]QuotaLimitStatus, 0, len(limits))
	for _, l := range limits {
		if MatchLimitModel(l, actualModel) {
			if strings.HasPrefix(l.AvailabilityGroup, antigravityPoolAvailabilityGroupPrefix) {
				l.AvailabilityGroup = ""
			}
			filtered = append(filtered, l)
		}
	}
	return filtered
}
