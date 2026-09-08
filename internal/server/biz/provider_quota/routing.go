package provider_quota

import (
	"time"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
)

type QuotaRoutingMode string

const (
	QuotaRoutingModeIgnoreQuota       QuotaRoutingMode = "ignore_quota"
	QuotaRoutingModeRemoveOnExhausted QuotaRoutingMode = "remove_on_exhausted"
	QuotaRoutingModeBackpressure      QuotaRoutingMode = "backpressure"
)

type RoutingState string

const (
	RoutingOpen       RoutingState = "open"
	RoutingStickyOnly RoutingState = "sticky_only"
	RoutingExhausted  RoutingState = "exhausted"
	RoutingUnknown    RoutingState = "unknown"
)

const (
	routingStatusExhausted = "exhausted"
	routingStatusAvailable = "available"
	routingStatusWarning   = "warning"
)

func IsBalanceLimit(l QuotaLimitStatus) bool {
	return l.Window == QuotaWindowPayAsYouGo || l.Window == QuotaWindowCredits
}

func ElapsedRatio(l QuotaLimitStatus, now time.Time) (float64, bool) {
	if l.PeriodStart == nil || l.NextResetAt == nil ||
		!l.PeriodStart.Before(now) || !now.Before(*l.NextResetAt) ||
		!l.NextResetAt.After(*l.PeriodStart) {
		return 0, false
	}

	ratio := float64(now.Sub(*l.PeriodStart)) / float64(l.NextResetAt.Sub(*l.PeriodStart))
	if ratio < 0 {
		return 0, true
	}
	if ratio > 1 {
		return 1, true
	}
	return float64(ratio), true
}

func EvaluateQuotaRouting(
	limits []QuotaLimitStatus,
	overallStatus string,
	limitType QuotaLimitType,
	now time.Time,
) (RoutingState, string) {
	if overallStatus == routingStatusExhausted {
		return RoutingExhausted, ""
	}

	groups := routingGroups(limits, limitType)
	if len(groups) == 0 {
		if overallStatus == routingStatusAvailable || overallStatus == routingStatusWarning {
			return RoutingOpen, ""
		}
		return RoutingUnknown, ""
	}

	allGroupsExhausted := true
	hasWindow := false
	allWindowsExhausted := true
	balanceAvailable := false
	for _, group := range groups {
		groupAvailable := false
		for _, limit := range group {
			if !routingLimitExhausted(limit) {
				groupAvailable = true
			}

			if IsBalanceLimit(limit) {
				if !routingLimitExhausted(limit) {
					balanceAvailable = true
				}
				continue
			}

			hasWindow = true
			if !routingLimitExhausted(limit) {
				allWindowsExhausted = false
			}
		}

		if groupAvailable {
			allGroupsExhausted = false
		}
	}

	if allGroupsExhausted {
		return RoutingExhausted, ""
	}

	if hasWindow && allWindowsExhausted && balanceAvailable {
		return RoutingStickyOnly, "window_exhausted_balance_fallback"
	}

	for _, group := range groups {
		for _, limit := range group {
			if IsBalanceLimit(limit) || routingLimitExhausted(limit) {
				continue
			}
			elapsedRatio, ok := ElapsedRatio(limit, now)
			if ok && limit.UsageRatio > elapsedRatio {
				return RoutingStickyOnly, "window_pressure"
			}
		}
	}

	return RoutingOpen, ""
}

func routingLimitExhausted(limit QuotaLimitStatus) bool {
	return limit.Status == routingStatusExhausted || limit.UsageRatio >= 1
}

func routingGroups(limits []QuotaLimitStatus, limitType QuotaLimitType) [][]QuotaLimitStatus {
	groupNames := make(map[string]bool)
	for _, limit := range limits {
		if limit.Type == limitType && limit.AvailabilityGroup != "" {
			groupNames[limit.AvailabilityGroup] = true
		}
	}

	grouped := make(map[string][]QuotaLimitStatus)
	var ungrouped []QuotaLimitStatus
	for _, limit := range limits {
		if limit.Type != limitType && !groupNames[limit.AvailabilityGroup] {
			continue
		}

		if groupNames[limit.AvailabilityGroup] {
			grouped[limit.AvailabilityGroup] = append(grouped[limit.AvailabilityGroup], limit)
			continue
		}
		ungrouped = append(ungrouped, limit)
	}

	groups := make([][]QuotaLimitStatus, 0, len(grouped)+len(ungrouped))
	for _, group := range grouped {
		groups = append(groups, group)
	}
	for _, limit := range ungrouped {
		groups = append(groups, []QuotaLimitStatus{limit})
	}
	return groups
}

func EffectiveStatus(
	limits []QuotaLimitStatus,
	overallStatus providerquotastatus.Status,
	overallReady bool,
	limitType QuotaLimitType,
) (providerquotastatus.Status, bool) {
	if overallStatus == providerquotastatus.StatusExhausted {
		return providerquotastatus.StatusExhausted, false
	}

	if len(limits) == 0 {
		return overallStatus, overallReady
	}

	var worstStatus providerquotastatus.Status
	worstReady := true
	found := false
	groupNames := make(map[string]bool)
	grouped := make(map[string][]QuotaLimitStatus)

	for _, limit := range limits {
		if limit.Type != limitType || limit.AvailabilityGroup == "" {
			continue
		}
		groupNames[limit.AvailabilityGroup] = true
	}

	for _, limit := range limits {
		if limit.Type != limitType && !groupNames[limit.AvailabilityGroup] {
			continue
		}

		if groupNames[limit.AvailabilityGroup] {
			grouped[limit.AvailabilityGroup] = append(grouped[limit.AvailabilityGroup], limit)
			continue
		}

		limitStatus := providerquotastatus.Status(limit.Status)
		if !found || quotaStatusRank(string(limitStatus)) > quotaStatusRank(string(worstStatus)) {
			worstStatus = limitStatus
			worstReady = limit.Ready
			found = true
		} else if quotaStatusRank(string(limitStatus)) == quotaStatusRank(string(worstStatus)) {
			worstReady = worstReady && limit.Ready
		}
	}

	for _, group := range grouped {
		bestStatus := providerquotastatus.StatusUnknown
		bestReady := false
		groupFound := false
		for _, limit := range group {
			limitStatus := providerquotastatus.Status(limit.Status)
			if !groupFound ||
				(limit.Ready && !bestReady) ||
				(limit.Ready == bestReady && quotaStatusRank(string(limitStatus)) < quotaStatusRank(string(bestStatus))) {
				bestStatus = limitStatus
				bestReady = limit.Ready
				groupFound = true
			} else if quotaStatusRank(string(limitStatus)) == quotaStatusRank(string(bestStatus)) {
				bestReady = bestReady || limit.Ready
			}
		}

		if !found || quotaStatusRank(string(bestStatus)) > quotaStatusRank(string(worstStatus)) {
			worstStatus = bestStatus
			worstReady = bestReady
			found = true
		} else if quotaStatusRank(string(bestStatus)) == quotaStatusRank(string(worstStatus)) {
			worstReady = worstReady && bestReady
		}
	}

	if !found {
		return providerquotastatus.StatusUnknown, true
	}

	return worstStatus, worstReady
}
