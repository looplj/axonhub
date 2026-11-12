// 职责: 为按 API key 划分独立额度账户的提供商提供共享的逐 key 采集与聚合。
// 关联文件:
//
//	internal/server/biz/provider_quota/types.go
//	internal/server/biz/provider_quota/opencode_go_checker.go
//	internal/server/biz/provider_quota/zhipu_checker.go
//	internal/server/biz/provider_quota/deepseek_checker.go
//	internal/server/biz/provider_quota/normalization.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	聚合结果里的 AvailabilityGroup 让路由对一个渠道的多个 key 做 OR，
//	单个 key 内部的多个窗口仍然 AND。binding limit 不能带上渠道级价格字段。

package provider_quota

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

// APIKeyQuotaErrorCheckFailed marks a key whose quota could not be read. The
// account row keeps no provider payload or error text; the wrapped error stays
// server-side for the caller's error classification.
const APIKeyQuotaErrorCheckFailed = "check_failed"

// apiKeyAccountsAvailabilityGroup tags the binding limit of every API key of a
// multi-key channel. EvaluateQuotaRouting ORs the members of one availability
// group, so a channel backed by several accounts stays routable while at least
// one key still has quota, while the windows inside one key keep their AND.
const apiKeyAccountsAvailabilityGroup = "api_key_accounts"

// apiKeyQuotaUnknownBindingProviders lists the providers whose per-key view
// publishes an explicit unknown binding limit for an enabled key it could not
// read, so an unreadable alternative is not reported as exhausted capacity.
// DeepSeek is excluded: it bills a balance, reports no window, and its
// account-derived status already carries the OR/unknown behavior.
var apiKeyQuotaUnknownBindingProviders = map[string]struct{}{
	"opencode_go": {},
	"zhipu":       {},
	"zai":         {},
}

// apiKeyQuotaTarget is one configured API key with its stable display ordinal.
type apiKeyQuotaTarget struct {
	Key   string
	Index int
}

// CollectAPIKeyQuotas queries the provider once per configured API key and
// folds the answers into one channel payload. Every configured key is queried,
// including keys that are disabled locally, so the per-key view stays complete;
// disabled keys only stop counting as serving capacity.
//
// A partial failure returns the other results plus unknown rows. When no key
// could be read the aggregated payload is still returned, together with a
// wrapped error so the caller can apply its usual error backoff.
func CollectAPIKeyQuotas(
	ctx context.Context,
	ch *ent.Channel,
	providerType string,
	now time.Time,
	fetch func(context.Context, *ent.Channel, string) (QuotaData, error),
) (QuotaData, error) {
	targets := apiKeyQuotaTargets(ch)
	if len(targets) == 0 {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	records := activeDisabledKeys(ch)
	accounts := make([]APIKeyQuota, 0, len(targets))

	var (
		failures int
		firstErr error
	)

	for _, target := range targets {
		account := APIKeyQuota{
			Key:    target.Key,
			Ref:    apiKeyQuotaRef(target.Key),
			Index:  target.Index,
			Suffix: apiKeyQuotaSuffix(target.Key),
		}
		if record, ok := records[target.Key]; ok {
			account.Disabled = true
			account.QuotaDisabled = record.QuotaDisabled
		}

		// The trim only applies to the outbound request: the stored disable
		// identity stays the configured string.
		data, err := fetch(ctx, ch, strings.TrimSpace(target.Key))
		if err != nil {
			failures++
			if firstErr == nil {
				firstErr = err
			}

			account.ErrorCode = APIKeyQuotaErrorCheckFailed
			account.Data = QuotaData{Status: "unknown", ProviderType: providerType}
			accounts = append(accounts, account)

			continue
		}

		data = NormalizeQuotaData(data)
		data.APIKeyQuotas = nil
		account.Data = data
		accounts = append(accounts, account)
	}

	aggregated := AggregateAPIKeyQuotas(providerType, accounts, now)
	if failures == len(accounts) {
		return aggregated, fmt.Errorf("%s quota check failed for all %d keys: %w", providerType, len(accounts), firstErr)
	}

	return aggregated, nil
}

// AggregateAPIKeyQuotas folds per-key results into the channel payload: every
// account stays listed, while the channel capacity comes from locally enabled
// accounts only.
func AggregateAPIKeyQuotas(providerType string, accounts []APIKeyQuota, now time.Time) QuotaData {
	status := aggregateAPIKeyQuotaStatus(accounts)
	data := QuotaData{
		Status:       status,
		ProviderType: providerType,
		Ready:        IsReadyStatus(status),
		Limits:       aggregateAPIKeyQuotaLimits(providerType, accounts, now),
		Balances:     aggregateAPIKeyQuotaBalances(accounts),
		NextResetAt:  aggregateAPIKeyQuotaReset(accounts),
		APIKeyQuotas: accounts,
	}

	// A lone key keeps the provider fields it always published, so stored
	// single-account consumers and the provider detail branch keep working.
	if len(accounts) == 1 {
		data.RawData = lo.Assign(map[string]any{}, accounts[0].Data.RawData)
	}

	return NormalizeQuotaData(data)
}

// aggregateAPIKeyQuotaStatus derives the channel status from the serving keys:
// one ready key keeps the channel usable, an unreadable key makes it unknown
// only when no ready key remains, and a pool whose every alternative is
// exhausted or disabled locally is exhausted. normalizeQuotaDataAt reuses this,
// so the service's unconditional normalization cannot turn a
// exhausted-plus-unknown pool into exhausted.
func aggregateAPIKeyQuotaStatus(accounts []APIKeyQuota) string {
	best := ""
	hasUnknown := false

	for _, account := range accounts {
		if account.Disabled {
			continue
		}

		status := account.Data.Status
		if account.ErrorCode != "" || status == "" || status == "unknown" {
			hasUnknown = true
			continue
		}
		if !IsReadyStatus(status) {
			continue
		}
		if best == "" || normalizedQuotaStatusRank(status) > normalizedQuotaStatusRank(best) {
			best = status
		}
	}

	if best != "" {
		return best
	}
	if hasUnknown {
		return "unknown"
	}

	return "exhausted"
}

// aggregateAPIKeyQuotaLimits publishes the routing limits of the channel.
// A single key keeps the per-window shape it always had; several keys publish
// one binding limit each inside a shared availability group.
func aggregateAPIKeyQuotaLimits(providerType string, accounts []APIKeyQuota, now time.Time) []QuotaLimitStatus {
	if len(accounts) == 1 {
		if limits := accounts[0].Data.Limits; len(limits) > 0 {
			clones := make([]QuotaLimitStatus, len(limits))
			copy(clones, limits)

			return clones
		}
	}

	limits := make([]QuotaLimitStatus, 0, len(accounts))

	for _, account := range accounts {
		if account.Disabled {
			continue
		}

		label := apiKeyQuotaLabel(account)
		if binding, ok := apiKeyBindingLimit(account, now); ok {
			binding.Account = label
			binding.AvailabilityGroup = apiKeyAccountsAvailabilityGroup
			limits = append(limits, binding)

			continue
		}

		if _, ok := apiKeyQuotaUnknownBindingProviders[providerType]; ok && account.ErrorCode != "" {
			limits = append(limits, QuotaLimitStatus{
				Type:              QuotaLimitTypeToken,
				Status:            "unknown",
				UsageRatio:        0,
				Ready:             false,
				Window:            QuotaWindowPrimary,
				AvailabilityGroup: apiKeyAccountsAvailabilityGroup,
				Account:           label,
			})
		}
	}

	return limits
}

// apiKeyBindingLimit returns the window that limits one key most. Usage-log
// spend is recorded per channel, so the binding copy drops the money fields
// while keeping the period bounds ElapsedRatio needs for BACKPRESSURE.
func apiKeyBindingLimit(account APIKeyQuota, now time.Time) (QuotaLimitStatus, bool) {
	var (
		binding QuotaLimitStatus
		found   bool
	)

	for _, limit := range account.Data.Limits {
		if limit.Type != QuotaLimitTypeToken || IsBalanceLimit(limit) {
			continue
		}
		if !found || apiKeyBindingLimitOutranks(limit, binding, now) {
			binding = limit
			found = true
		}
	}
	if !found {
		return QuotaLimitStatus{}, false
	}

	binding.PeriodCost = nil
	binding.PeriodQuota = nil

	return binding, true
}

// apiKeyBindingLimitOutranks reports whether candidate limits the key more than
// current: an exhausted window first, then a window whose usage is ahead of its
// elapsed time, then the highest usage ratio. The original window order wins
// ties, which is what keeps the selection stable while ratios move.
func apiKeyBindingLimitOutranks(candidate, current QuotaLimitStatus, now time.Time) bool {
	candidateExhausted, candidatePressure := apiKeyBindingLimitFlags(candidate, now)
	currentExhausted, currentPressure := apiKeyBindingLimitFlags(current, now)

	if candidateExhausted != currentExhausted {
		return candidateExhausted
	}
	if candidatePressure != currentPressure {
		return candidatePressure
	}

	return candidate.UsageRatio > current.UsageRatio
}

func apiKeyBindingLimitFlags(limit QuotaLimitStatus, now time.Time) (exhausted, pressure bool) {
	if limit.Status == "exhausted" || limit.UsageRatio >= 1 {
		exhausted = true
	}
	if elapsed, ok := ElapsedRatio(limit, now); ok && limit.UsageRatio > elapsed {
		pressure = true
	}

	return exhausted, pressure
}

// aggregateAPIKeyQuotaBalances flattens the per-key money rows. Balances of
// different currencies or keys are never summed: each row keeps the account it
// belongs to so the UI can attribute it.
func aggregateAPIKeyQuotaBalances(accounts []APIKeyQuota) []QuotaBalance {
	if len(accounts) == 0 {
		return nil
	}

	labelAccounts := len(accounts) > 1
	balances := make([]QuotaBalance, 0, len(accounts))

	for _, account := range accounts {
		label := ""
		if labelAccounts {
			label = apiKeyQuotaLabel(account)
		}

		for _, balance := range account.Data.Balances {
			row := balance
			if label != "" {
				row.Account = label
			}

			balances = append(balances, row)
		}
	}

	if len(balances) == 0 {
		return nil
	}

	return balances
}

// aggregateAPIKeyQuotaReset reports the earliest provider reset among the
// serving keys. It is an observation, never an automatic re-enable deadline.
func aggregateAPIKeyQuotaReset(accounts []APIKeyQuota) *time.Time {
	var earliest *time.Time

	observe := func(candidate *time.Time) {
		if candidate == nil || candidate.IsZero() {
			return
		}
		if earliest == nil || candidate.Before(*earliest) {
			earliest = lo.ToPtr(*candidate)
		}
	}

	for _, account := range accounts {
		if account.Disabled {
			continue
		}

		observe(account.Data.NextResetAt)
		for _, limit := range account.Data.Limits {
			observe(limit.NextResetAt)
		}
	}

	return earliest
}

// apiKeyQuotaLabel builds the collision-free display label of one account. It
// never contains more than the four trailing characters of the key.
func apiKeyQuotaLabel(account APIKeyQuota) string {
	if account.Suffix == "" {
		return fmt.Sprintf("#%d", account.Index)
	}

	return fmt.Sprintf("#%d ****%s", account.Index, account.Suffix)
}

// apiKeyQuotaTargets enumerates the channel's API keys in their configured
// order, skipping blank entries and exact duplicates. Index keeps the ordinal
// of GetAllAPIKeys so a key keeps its number across checks.
func apiKeyQuotaTargets(ch *ent.Channel) []apiKeyQuotaTarget {
	candidates := ch.Credentials.GetAllAPIKeys()
	targets := make([]apiKeyQuotaTarget, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))

	for position, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}

		seen[candidate] = struct{}{}
		targets = append(targets, apiKeyQuotaTarget{Key: candidate, Index: position + 1})
	}

	return targets
}

// activeDisabledKeys returns the active disable records of the channel, keyed
// by the configured key. Expired records disable nothing.
func activeDisabledKeys(ch *ent.Channel) map[string]objects.DisabledAPIKey {
	records := make(map[string]objects.DisabledAPIKey, len(ch.DisabledAPIKeys))

	for _, record := range ch.DisabledAPIKeys {
		if strings.TrimSpace(record.Key) == "" || record.IsExpired() {
			continue
		}

		records[record.Key] = record
	}

	return records
}

// apiKeyQuotaRef is the stable, non-reversible identity of a key inside the
// payload: the full SHA-256 digest of the configured key.
func apiKeyQuotaRef(key string) string {
	digest := sha256.Sum256([]byte(key))

	return hex.EncodeToString(digest[:])
}

// apiKeyQuotaSuffix is the trailing fragment the channel UI masks keys with. A
// key short enough to be recognizable on its own gets no suffix at all.
func apiKeyQuotaSuffix(key string) string {
	runes := []rune(key)
	if len(runes) <= 4 {
		return ""
	}

	return string(runes[len(runes)-4:])
}
