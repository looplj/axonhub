package provider_quota

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type antigravitySummary struct {
	Groups []struct {
		DisplayName string `json:"displayName"`
		Name        string `json:"name"`
		Buckets     []struct {
			BucketID          string   `json:"bucketId"`
			ID                string   `json:"id"`
			DisplayName       string   `json:"displayName"`
			Window            string   `json:"window"`
			Disabled          bool     `json:"disabled"`
			RemainingFraction *float64 `json:"remainingFraction"`
			Remaining         *struct {
				RemainingFraction *float64 `json:"remainingFraction"`
				Case              string   `json:"case"`
				Value             *float64 `json:"value"`
			} `json:"remaining"`
			ResetTime any `json:"resetTime"`
		} `json:"buckets"`
	} `json:"groups"`
}

// antigravityModelPool identifies the two provider quota pools without merging their percentages.
func antigravityModelPool(id string) string {
	id = strings.ToLower(id)
	if strings.Contains(id, "gemini") {
		return "gemini"
	}
	if strings.Contains(id, "claude") || strings.Contains(id, "gpt") || strings.HasPrefix(id, "3p-") {
		return "claude_gpt"
	}
	return ""
}

// antigravityPoolAvailabilityGroupPrefix names the capacity source behind each
// Antigravity pool. Gemini and Claude/GPT are metered against separate provider
// quotas that reset independently, so a model of one pool must stay routable
// while only a limit of the other pool is exhausted. Tagging each pool's limits
// with its own group lets EvaluateQuotaRouting apply OR semantics inside a pool
// while keeping the two pools independent. Without it every ungrouped pool
// window collapses into one implicit group where a single exhausted window (for
// example a spent Claude/GPT weekly bucket) poisons the whole channel, including
// Gemini models.
const antigravityPoolAvailabilityGroupPrefix = "antigravity_pool:"

// antigravityPoolAvailabilityGroup returns the availability group of a pool. It
// returns an empty string for an unknown pool so such limits keep the legacy
// ungrouped evaluation instead of joining a pool they do not belong to.
func antigravityPoolAvailabilityGroup(pool string) string {
	if pool == "" {
		return ""
	}
	return antigravityPoolAvailabilityGroupPrefix + pool
}

// antigravityCadenceTokens splits a provider-supplied cadence, bucket ID or
// display name into lower-case alphanumeric tokens. Only complete tokens are
// matched later, because substring matching misreads near misses such as
// "15h" or "biweekly".
func antigravityCadenceTokens(value string) []string {
	return strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
}

// antigravitySummaryWindow recognizes only cadences the provider names, never reset-time guesses.
func antigravitySummaryWindow(window, id, displayName string) (string, time.Duration) {
	for _, value := range []string{window, id, displayName} {
		tokens := antigravityCadenceTokens(value)
		for i, token := range tokens {
			switch {
			case token == "7d" || token == "week" || token == "weekly":
				// "bi-weekly" and "semi-weekly" name a different cadence.
				if i > 0 && (tokens[i-1] == "bi" || tokens[i-1] == "semi") {
					continue
				}
				return "7d", 7 * 24 * time.Hour
			case token == "5h" || token == "session":
				return "5h", 5 * time.Hour
			case i+1 < len(tokens) && (token == "5" || token == "five") && tokens[i+1] == "hour":
				return "5h", 5 * time.Hour
			case i+1 < len(tokens) && (token == "7" || token == "seven") && tokens[i+1] == "day":
				return "7d", 7 * 24 * time.Hour
			}
		}
	}
	return "", 0
}

// parseAntigravityQuotaSummary preserves the upstream payload and reports one limit per pool and cadence.
func parseAntigravityQuotaSummary(body []byte) (QuotaData, error) {
	var response struct {
		antigravitySummary

		Response *antigravitySummary `json:"response"`
		Summary  *antigravitySummary `json:"summary"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("decode Antigravity quota summary: %w", err)
	}
	summary := response.antigravitySummary
	if response.Response != nil {
		summary = *response.Response
	} else if response.Summary != nil {
		summary = *response.Summary
	}
	byWindow := make(map[string]QuotaLimitStatus)
	for _, group := range summary.Groups {
		for _, bucket := range group.Buckets {
			id := bucket.BucketID
			if id == "" {
				id = bucket.ID
			}
			pool := antigravityModelPool(id + " " + group.DisplayName + " " + group.Name)
			window, duration := antigravitySummaryWindow(bucket.Window, id, bucket.DisplayName)
			remaining := bucket.RemainingFraction
			if remaining == nil && bucket.Remaining != nil {
				remaining = bucket.Remaining.RemainingFraction
				if remaining == nil && bucket.Remaining.Case == "remainingFraction" {
					remaining = bucket.Remaining.Value
				}
			}
			if bucket.Disabled || pool == "" || window == "" || remaining == nil ||
				math.IsNaN(*remaining) || math.IsInf(*remaining, 0) || *remaining < 0 || *remaining > 1 {
				continue
			}
			key := pool + "_" + window
			usage := 1 - *remaining
			if existing, ok := byWindow[key]; !ok || usage > existing.UsageRatio {
				limit := NewTokenLimitStatus(antigravityQuotaStatus(usage), usage, parseAntigravityResetTime(bucket.ResetTime)).WithWindow(key, duration)
				limit.AvailabilityGroup = antigravityPoolAvailabilityGroup(pool)
				byWindow[key] = limit
			}
		}
	}
	limits := make([]QuotaLimitStatus, 0, 4)
	bestRemaining := 0.0
	for _, pool := range []string{"gemini", "claude_gpt"} {
		poolRemaining := 1.0
		found := false
		for _, window := range []string{"5h", "7d"} {
			if limit, ok := byWindow[pool+"_"+window]; ok {
				limits = append(limits, limit)
				poolRemaining = min(poolRemaining, 1-limit.UsageRatio)
				found = true
			}
		}
		if found {
			bestRemaining = max(bestRemaining, poolRemaining)
		}
	}
	if len(limits) == 0 {
		return QuotaData{}, fmt.Errorf("Antigravity quota summary has no measured pool windows")
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return QuotaData{}, fmt.Errorf("preserve Antigravity quota summary: %w", err)
	}
	return NormalizeQuotaData(QuotaData{Status: antigravityQuotaStatus(1 - bestRemaining), ProviderType: "antigravity", RawData: raw, Limits: limits}), nil
}
