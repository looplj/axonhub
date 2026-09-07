package provider_quota

import (
	"math"
	"time"
)

// NormalizeQuotaData applies the common provider quota contract at the checker boundary.
func NormalizeQuotaData(data QuotaData) QuotaData {
	return normalizeQuotaDataAt(data, time.Now())
}

func normalizeQuotaDataAt(data QuotaData, now time.Time) QuotaData {
	data.Limits = normalizeQuotaLimits(data.Limits, now)

	status := normalizeOverallQuotaStatus(data.Status, data.Limits)

	var nextResetAt *time.Time
	if data.NextResetAt != nil && data.NextResetAt.After(now) {
		resetAt := *data.NextResetAt
		nextResetAt = &resetAt
	}
	for _, limit := range data.Limits {
		if limit.NextResetAt != nil && limit.NextResetAt.After(now) &&
			(nextResetAt == nil || limit.NextResetAt.Before(*nextResetAt)) {
			resetAt := *limit.NextResetAt
			nextResetAt = &resetAt
		}
	}

	data.Status = status
	data.NextResetAt = nextResetAt
	data.Ready = IsReadyStatus(status)
	return data
}

func normalizeOverallQuotaStatus(explicitStatus string, limits []QuotaLimitStatus) string {
	status := normalizeQuotaStatus(explicitStatus)
	if status == "exhausted" {
		return status
	}

	bestReadyStatus := ""
	if IsReadyStatus(status) {
		bestReadyStatus = status
	}
	hasExhausted := false

	for _, limit := range limits {
		switch {
		case IsReadyStatus(limit.Status):
			if bestReadyStatus == "" || normalizedQuotaStatusRank(limit.Status) > normalizedQuotaStatusRank(bestReadyStatus) {
				bestReadyStatus = limit.Status
			}
		case limit.Status == "exhausted":
			hasExhausted = true
		}
	}

	if bestReadyStatus != "" {
		return bestReadyStatus
	}
	if hasExhausted {
		return "exhausted"
	}
	return "unknown"
}

func normalizeQuotaLimits(limits []QuotaLimitStatus, now time.Time) []QuotaLimitStatus {
	if len(limits) == 0 {
		return nil
	}

	normalized := make([]QuotaLimitStatus, 0, len(limits))
	indexes := make(map[string]int, len(limits))
	for _, limit := range limits {
		if limit.Type == "" || limit.Window == "" || !isFiniteRatio(limit.UsageRatio) || limit.UsageRatio < 0 {
			continue
		}

		limit.UsageRatio = min(limit.UsageRatio, 1)
		limit.Status = normalizeQuotaStatus(limit.Status)
		if limit.UsageRatio >= 1 {
			limit.Status = "exhausted"
		}
		limit.Ready = IsReadyStatus(limit.Status)
		if limit.NextResetAt != nil && limit.NextResetAt.IsZero() {
			limit.NextResetAt = nil
		}
		if limit.NextResetAt != nil && !limit.NextResetAt.After(now) {
			limit.NextResetAt = nil
		}
		if limit.NextResetAt == nil {
			limit.PeriodStart = nil
		} else if limit.PeriodStart != nil && !limit.PeriodStart.Before(*limit.NextResetAt) {
			limit.PeriodStart = nil
		}

		identity := string(limit.Type) + "\x00" + limit.Window
		if index, ok := indexes[identity]; ok {
			mergeQuotaLimit(&normalized[index], limit)
			continue
		}
		indexes[identity] = len(normalized)
		normalized = append(normalized, limit)
	}

	return normalized
}

func mergeQuotaLimit(existing *QuotaLimitStatus, incoming QuotaLimitStatus) {
	if incoming.UsageRatio > existing.UsageRatio {
		existing.UsageRatio = incoming.UsageRatio
	}
	if normalizedQuotaStatusRank(incoming.Status) > normalizedQuotaStatusRank(existing.Status) {
		existing.Status = incoming.Status
	}
	if incoming.NextResetAt != nil &&
		(existing.NextResetAt == nil || incoming.NextResetAt.Before(*existing.NextResetAt)) {
		resetAt := *incoming.NextResetAt
		existing.NextResetAt = &resetAt
	}
	if incoming.PeriodStart != nil &&
		(existing.PeriodStart == nil || incoming.PeriodStart.Before(*existing.PeriodStart)) {
		periodStart := *incoming.PeriodStart
		existing.PeriodStart = &periodStart
	}
	if existing.NextResetAt == nil || existing.PeriodStart == nil ||
		!existing.PeriodStart.Before(*existing.NextResetAt) {
		existing.PeriodStart = nil
	}
	existing.Ready = IsReadyStatus(existing.Status)
}

func isFiniteRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0)
}

func normalizeQuotaStatus(status string) string {
	switch status {
	case "available", "warning", "exhausted", "unknown":
		return status
	default:
		return "unknown"
	}
}

func normalizedQuotaStatusRank(status string) int {
	switch status {
	case "exhausted":
		return 3
	case "warning":
		return 2
	case "available":
		return 1
	default:
		return 0
	}
}
