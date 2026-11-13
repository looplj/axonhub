package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/internal/server/scheduler"
	"github.com/looplj/axonhub/llm/httpclient"
)

const maxConcurrentQuotaChecks = 8

// Quota check failures back off exponentially so a persistently failing channel
// (expired credentials, or a scraped provider dashboard whose markup changed) is
// retried at a slow cadence instead of on every check interval. This mirrors the
// model circuit breaker's probe backoff (see model_circuit_breaker.go).
const (
	quotaErrorCodeCheckFailed        = "check_failed"
	quotaErrorCodeMissingCredentials = "missing_credentials"

	// maxQuotaErrorBackoffMultiplier caps the backoff growth at 8x the base
	// interval, matching the circuit breaker's probe backoff cap.
	maxQuotaErrorBackoffMultiplier = 8

	// maxQuotaErrorBackoffSteps is the consecutive-failure count at which the
	// multiplier saturates (1, 2, 4, 8). The persisted error_count is clamped
	// here so it stays bounded for a channel that never recovers.
	maxQuotaErrorBackoffSteps = 4
)

func quotaErrorCode(err error) string {
	if err != nil && err.Error() == "channel has no credentials" {
		return quotaErrorCodeMissingCredentials
	}

	return quotaErrorCodeCheckFailed
}

var providerQuotaChannelTypes = []channel.Type{
	channel.TypeClaudecode,
	channel.TypeCodex,
	channel.TypeAntigravity,
	channel.TypeXaiSubscription,
	channel.TypeGithubCopilot,
	channel.TypeNanogpt,
	channel.TypeNanogptResponses,
	channel.TypeZenmux,
	channel.TypeZenmuxResponses,
	channel.TypeZenmuxAnthropic,
	channel.TypeZenmuxGemini,
	channel.TypeZenmuxVideo,
	channel.TypeCline,
	channel.TypeOpenai,
	channel.TypeOpenaiResponses,
	channel.TypeOpencodeGo,
	channel.TypeOpencodeGoAnthropic,
	channel.TypeMoonshotCoding,
	channel.TypeMinimax,
	channel.TypeMinimaxAnthropic,
	channel.TypeZhipu,
	channel.TypeZhipuAnthropic,
	channel.TypeZai,
	channel.TypeZaiAnthropic,
	channel.TypeDeepseek,
	channel.TypeDeepseekAnthropic,
	channel.TypeCommandcode,
	channel.TypeCommandcodeAnthropic,
	channel.TypeOllama,
	channel.TypeOllamaAnthropic,
}

// quotaErrorBackoff returns the next-check delay after `failures` consecutive
// quota check failures: base, 2x, 4x, ... capped at maxQuotaErrorBackoffMultiplier.
// A successful check clears the counter (saveQuotaStatus overwrites quota_data),
// so the cadence returns to base as soon as the provider recovers.
func quotaErrorBackoff(base time.Duration, failures int) time.Duration {
	multiplier := 1
	for i := 1; i < failures && multiplier < maxQuotaErrorBackoffMultiplier; i++ {
		multiplier *= 2
	}

	if multiplier > maxQuotaErrorBackoffMultiplier {
		multiplier = maxQuotaErrorBackoffMultiplier
	}

	return base * time.Duration(multiplier)
}

// quotaErrorCount reads the persisted consecutive-failure counter from quota_data.
// Values stored in-process are int; values reloaded from the DB are float64.
func quotaErrorCount(data map[string]any) int {
	switch v := data["error_count"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// nextQuotaErrorCount increments the consecutive-failure counter, clamped at
// maxQuotaErrorBackoffSteps so the value persisted in quota_data stays bounded
// once the backoff multiplier has saturated.
func nextQuotaErrorCount(prev int) int {
	if prev+1 > maxQuotaErrorBackoffSteps {
		return maxQuotaErrorBackoffSteps
	}

	return prev + 1
}

type QuotaChannelStatus struct {
	ProviderType string
	Status       providerquotastatus.Status
	Ready        bool
	Limits       []provider_quota.QuotaLimitStatus

	// LastKnownStatus is the newest non-unknown status persisted by this
	// service for the channel: the status itself when it is not `unknown`,
	// otherwise the `_previous_status` carried in the persisted quota_data.
	// Every save records it, so it stays monotonically fresh — which is what
	// a failed-reload refresh resolves its webhook transition from when the
	// selected previous status is `unknown` (the stale pre-reset channel
	// edge's carry must not win).
	LastKnownStatus string
}

func cloneQuotaLimitStatus(limit provider_quota.QuotaLimitStatus) provider_quota.QuotaLimitStatus {
	clone := limit
	if limit.NextResetAt != nil {
		clone.NextResetAt = lo.ToPtr(*limit.NextResetAt)
	}
	if limit.PeriodStart != nil {
		clone.PeriodStart = lo.ToPtr(*limit.PeriodStart)
	}
	if limit.PeriodCost != nil {
		clone.PeriodCost = lo.ToPtr(*limit.PeriodCost)
	}
	if limit.PeriodQuota != nil {
		clone.PeriodQuota = lo.ToPtr(*limit.PeriodQuota)
	}
	return clone
}

func cloneLimits(limits []provider_quota.QuotaLimitStatus) []provider_quota.QuotaLimitStatus {
	if limits == nil {
		return nil
	}

	clones := make([]provider_quota.QuotaLimitStatus, len(limits))
	for i, limit := range limits {
		clones[i] = cloneQuotaLimitStatus(limit)
	}
	return clones
}

// cloneBalances deep-copies balances so account-group members do not share
// mutable optional fields.
func cloneBalances(balances []provider_quota.QuotaBalance) []provider_quota.QuotaBalance {
	if balances == nil {
		return nil
	}

	clones := make([]provider_quota.QuotaBalance, len(balances))
	for i, balance := range balances {
		clone := balance
		if balance.Granted != nil {
			clone.Granted = lo.ToPtr(*balance.Granted)
		}
		if balance.ToppedUp != nil {
			clone.ToppedUp = lo.ToPtr(*balance.ToppedUp)
		}
		clones[i] = clone
	}
	return clones
}

// EffectiveStatus returns the effective quota status for the given limit type.
//
// If the channel-level status is Exhausted, it short-circuits regardless of
// per-limit data — a channel marked exhausted at the top level is treated as
// fully unavailable. This means if a future provider sets channel-level
// "exhausted" for a single limit type (e.g., images), token-limit queries
// would also return "exhausted" even if tokens remain.
func (s *QuotaChannelStatus) EffectiveStatus(limitType provider_quota.QuotaLimitType) (providerquotastatus.Status, bool) {
	return provider_quota.EffectiveStatus(s.Limits, s.Status, s.Ready, limitType)
}

// HOW TO ADD A NEW PROVIDER QUOTA CHECKER
// ========================================
//
// There are two patterns depending on whether the provider has its own
// channel type or shares an existing OpenAI-compatible channel type:
//
// ── PATTERN A: Dedicated channel type (e.g. claudecode, codex, nanogpt) ──
//
// 1. Create the checker in internal/server/biz/provider_quota/
//
//    Implement the QuotaChecker interface:
//      - CheckQuota(ctx, ch) -> makes the API request and parses the response internally
//      - Returns normalized QuotaData with:
//        * Status: "available", "warning", "exhausted", or "unknown"
//        * Ready: true for available/warning, false for exhausted/unknown
//        * NextResetAt: optional timestamp of next quota reset
//        * RawData: provider-specific data (stored in JSON format)
//
//    When the provider reports (or the plan fixes) the length of a limit
//    window, label the limit with QuotaLimitStatus.WithWindow so it also
//    carries a PeriodStart. That is what lets the service price the period from
//    usage logs (see provider_quota_cost.go); limits without a period start
//    simply get no money estimate. Do NOT derive one from a timestamp that is
//    an incremental regeneration tick rather than a window boundary — see the
//    synthetic checker for that case.
//
//    Counters for capabilities AxonHub does not route (e.g. search requests)
//    are listed with a QuotaLimitType that no request modality maps to, are
//    registered in displayOnlyLimitTypes, and carry no AvailabilityGroup, so
//    they never gate routing or the channel status — only their own table row
//    and badge entry.
//
// 2. Add the provider type to the database schema
//
//    In internal/ent/schema/channel.go:
//      - Add new value to the channel.Type enum (e.g., "myprovider")
//
//    In internal/ent/schema/provider_quota_status.go:
//      - Add new value to the provider_type enum (e.g., "myprovider")
//
// 3. Register the provider in ProviderQuotaService and collection settings
//
//    a. Create a registration function (e.g., registerMyProviderSupport())
//    b. Add it to registerProviderQuotaSupport()
//    c. Add the provider key to supportedProviderQuotaTypes in provider_quota_settings.go
//    d. Add the provider name to both frontend system locale files under
//       system.quota.collection.providers
//    e. Update getProviderType() to map channel.TypeMyprovider -> "myprovider"
//    f. Update runQuotaCheck() to include channel.TypeMyprovider in TypeIn filter
//
//    Example:
//
//      func (svc *ProviderQuotaService) registerMyProviderSupport() {
//        svc.checkers["myprovider"] = provider_quota.NewMyProviderQuotaChecker(svc.httpClient)
//      }
//
// ── PATTERN B: URL-based detection for OpenAI-compatible providers ──
//
//    Use this pattern when the provider reuses channel.TypeOpenai or
//    channel.TypeOpenaiResponses but has its own quota API.
//
// 1. Create the checker in internal/server/biz/provider_quota/
//
//    Same QuotaChecker interface as Pattern A, plus:
//      - SupportsChannel(ch) must check URL (e.g., strings.HasSuffix(host, ".api.synthetic.new"))
//
// 2. Add the provider type to the database schema
//
//    In internal/ent/schema/provider_quota_status.go:
//      - Add new value to the provider_type enum (e.g., "synthetic")
//
//    Do NOT modify channel.Type enum — the provider reuses TypeOpenai.
//
// 3. Add URL detection in internal/server/biz/provider_quota/url_detection.go
//
//    a. Add the URL pattern to urlProviderMap (e.g., "api.synthetic.new": "synthetic")
//    b. The DetectProviderFromURL() function handles the mapping
//
// 4. Register the provider in ProviderQuotaService and collection settings
//
//    a. Create a registration function (e.g., registerSyntheticSupport())
//    b. Add it to registerProviderQuotaSupport()
//    c. Add the provider key to supportedProviderQuotaTypes in provider_quota_settings.go
//    d. Add the provider name to both frontend system locale files under
//       system.quota.collection.providers
//    e. getProviderType() already handles URL-based detection for TypeOpenai
//    f. hasCredentialsForProvider() already handles API-key-only auth for URL-detected providers
//
//    Example:
//
//      func (svc *ProviderQuotaService) registerSyntheticSupport() {
//        svc.checkers["synthetic"] = provider_quota.NewSyntheticQuotaChecker(svc.httpClient)
//      }
//
// 5. Regenerate Ent schema
//
//    make generate
//
// 6. Implement the frontend display (optional)
//
//    Add provider-specific display logic in frontend/src/components/quota-badges.tsx:
//      - Update QuotaData type to include provider-specific fields
//      - Add display logic for the provider type in QuotaRow component
//
// EXAMPLE: CLAUDE CODE PROVIDER (Pattern A)
// =========================================
//
// Checker: internal/server/biz/provider_quota/claudecode_checker.go
//   - Makes minimal request to Claude Code API
//   - Internally parses rate limit headers (anthropic-ratelimit-unified-status, etc.)
//   - Normalizes status (allowed -> available, throttled -> exhausted)
//   - Detects warning state (utilization >= 80%)
//   - Maps representative claim to reset time
//
// EXAMPLE: CODEX PROVIDER (Pattern A)
// ===================================
//
// Checker: internal/server/biz/provider_quota/codex_checker.go
//   - Makes request to ChatGPT usage endpoint (/backend-api/wham/usage)
//   - Internally parses JSON response (plan_type, rate_limit)
//   - Normalizes status based on limit_reached and allowed flags
//   - Detects warning state (primary_window.used_percent >= 80)
//
// EXAMPLE: NANO GPT PROVIDER (Pattern A, simple API key, non-OAuth)
// ================================================================
//
// Checker: internal/server/biz/provider_quota/nanogpt_checker.go
//   - Makes request to NanoGPT subscription usage endpoint (/api/subscription/v1/usage)
//   - Uses simple API key authentication (no OAuth required)
//   - Internally parses JSON response (state, windows, percentUsed)
//   - Normalizes status: active→available, grace→warning, inactive→exhausted
//   - Detects high-usage warning state (any window percentUsed >= 0.8)
//
// EXAMPLE: SYNTHETIC PROVIDER (Pattern B, URL-based detection)
// =============================================================
//
// Checker: internal/server/biz/provider_quota/synthetic_checker.go
//   - Reuses channel.TypeOpenai / TypeOpenaiResponses
//   - URL detection: host ending in ".api.synthetic.new" → provider_type "synthetic"
//   - Makes request to /v2/quotas endpoint
//   - Uses simple API key authentication (no OAuth required)
//   - Internally parses nested JSON (subscription, search, freeToolCalls, weeklyTokenLimit, rollingFiveHourLimit)
//   - Normalizes status: limited=true → exhausted, percentRemaining < 20 → warning, else → available
//   - The weekly limit carries the provider's own credit money (maxCredits / remainingCredits)
//
// EXAMPLE: NEURALWATT PROVIDER (Pattern B, URL-based detection)
// ==============================================================
//
// Checker: internal/server/biz/provider_quota/neuralwatt_checker.go
//   - Reuses channel.TypeOpenai / TypeOpenaiResponses
//   - URL detection: host ending in ".api.neuralwatt.com" → provider_type "neuralwatt"
//   - Makes request to /v1/quota endpoint
//   - Uses simple API key authentication (no OAuth required)
//   - Internally parses JSON (kwh_included, kwh_remaining, in_overage)
//   - Normalizes status: in_overage → exhausted, remaining < 20% → warning, else → available
//

type ProviderQuotaServiceParams struct {
	fx.In

	Ent                       *ent.Client
	SystemService             *SystemService
	HttpClient                *httpclient.HttpClient
	WebhookNotifier           *WebhookNotifier
	CheckInterval             time.Duration `name:"provider_quota_check_interval" optional:"true"`
	WarningCheckIntervalRatio int           `name:"provider_quota_warning_check_interval_ratio" optional:"true"`
}

type ProviderQuotaService struct {
	*AbstractService

	SystemService             *SystemService
	WebhookNotifier           *WebhookNotifier
	checkInterval             time.Duration
	warningCheckIntervalRatio int
	httpClient                *httpclient.HttpClient

	// Registry
	checkers map[string]provider_quota.QuotaChecker

	mu         sync.Mutex
	quotaCache sync.Map
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
	svc := &ProviderQuotaService{
		AbstractService:           &AbstractService{db: params.Ent},
		SystemService:             params.SystemService,
		checkers:                  make(map[string]provider_quota.QuotaChecker),
		checkInterval:             params.CheckInterval,
		warningCheckIntervalRatio: params.WarningCheckIntervalRatio,
		httpClient:                params.HttpClient,
		WebhookNotifier:           params.WebhookNotifier,
	}

	svc.registerProviderQuotaSupport()

	svc.loadQuotaCache(context.Background())

	return svc
}

func (svc *ProviderQuotaService) registerProviderQuotaSupport() {
	svc.registerClaudeCodeSupport()
	svc.registerCodexSupport()
	svc.registerAntigravitySupport()
	svc.registerXAISubscriptionSupport()
	svc.registerGithubCopilotSupport()
	svc.registerNanoGPTSupport()
	svc.registerZenmuxSupport()
	svc.registerClineSupport()
	svc.registerSyntheticSupport()
	svc.registerNeuralWattSupport()
	svc.registerApertisSupport()
	svc.registerOpenCodeGoSupport()
	svc.registerKimiCodeSupport()
	svc.registerMinimaxSupport()
	svc.registerZhipuSupport()
	svc.registerZaiSupport()
	svc.registerDeepSeekSupport()
	svc.registerCharmHyperSupport()
	svc.registerCommandCodeSupport()
	svc.registerOllamaSupport()
}

func (svc *ProviderQuotaService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	cronExpr := svc.intervalToCronExpr(svc.getCheckInterval())
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "provider-quota-check",
		Description: "Check provider quota usage periodically",
		CronExpr:    cronExpr,
		Timezone:    "UTC",
	}, svc.runQuotaCheckScheduled)
}

func (svc *ProviderQuotaService) registerClaudeCodeSupport() {
	svc.checkers["claudecode"] = provider_quota.NewClaudeCodeQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerCommandCodeSupport() {
	svc.checkers["commandcode"] = provider_quota.NewCommandCodeQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerOllamaSupport() {
	svc.checkers["ollama"] = provider_quota.NewOllamaQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerCodexSupport() {
	svc.checkers["codex"] = provider_quota.NewCodexQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerAntigravitySupport() {
	svc.checkers["antigravity"] = provider_quota.NewAntigravityQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerXAISubscriptionSupport() {
	svc.checkers["xai_subscription"] = provider_quota.NewXAISubscriptionQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerGithubCopilotSupport() {
	svc.checkers["github_copilot"] = provider_quota.NewGithubCopilotQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerNanoGPTSupport() {
	svc.checkers["nanogpt"] = provider_quota.NewNanoGPTQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerZenmuxSupport() {
	svc.checkers["zenmux"] = provider_quota.NewZenmuxQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerClineSupport() {
	svc.checkers["cline"] = provider_quota.NewClineQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerSyntheticSupport() {
	svc.checkers["synthetic"] = provider_quota.NewSyntheticQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerNeuralWattSupport() {
	svc.checkers["neuralwatt"] = provider_quota.NewNeuralWattQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerApertisSupport() {
	svc.checkers["apertis"] = provider_quota.NewApertisQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerOpenCodeGoSupport() {
	svc.checkers["opencode_go"] = provider_quota.NewOpenCodeGoQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerKimiCodeSupport() {
	svc.checkers["kimi_code"] = provider_quota.NewKimiCodeQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerMinimaxSupport() {
	svc.checkers["minimax"] = provider_quota.NewMinimaxQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerZhipuSupport() {
	svc.checkers["zhipu"] = provider_quota.NewZhipuQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerZaiSupport() {
	svc.checkers["zai"] = provider_quota.NewZaiQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerCharmHyperSupport() {
	svc.checkers["charm_hyper"] = provider_quota.NewCharmHyperQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerDeepSeekSupport() {
	svc.checkers["deepseek"] = provider_quota.NewDeepSeekQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) intervalToCronExpr(interval time.Duration) string {
	minutes := int(interval.Minutes())
	hours := int(interval.Hours())

	// Hourly or longer intervals
	if hours >= 1 && minutes%60 == 0 {
		if hours == 1 {
			return "0 * * * *" // Every hour
		}

		return fmt.Sprintf("0 */%d * * *", hours) // Every N hours
	}

	// Minute intervals that divide evenly into 60
	if minutes > 0 && 60%minutes == 0 {
		return fmt.Sprintf("*/%d * * * *", minutes)
	}

	// Round down to nearest supported interval (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)
	supportedIntervals := []int{1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60}
	filtered := lo.Filter(supportedIntervals, func(si int, _ int) bool {
		return si <= minutes
	})

	rounded := 60
	if len(filtered) > 0 {
		rounded = lo.Max(filtered)
	}

	log.Warn(context.Background(), "Quota check interval does not divide evenly into 60 minutes, rounding to nearest supported interval",
		log.Int("requested_minutes", minutes),
		log.Int("rounded_minutes", rounded))

	return fmt.Sprintf("*/%d * * * *", rounded)
}

func (svc *ProviderQuotaService) getWarningCheckInterval() time.Duration {
	ratio := svc.warningCheckIntervalRatio
	if ratio <= 0 {
		ratio = 4
	}

	return svc.getCheckInterval() * time.Duration(ratio)
}

func (svc *ProviderQuotaService) nextCheckIntervalForStatus(status providerquotastatus.Status) time.Duration {
	if status == providerquotastatus.StatusWarning {
		return svc.getWarningCheckInterval()
	}
	return svc.getCheckInterval()
}

func (svc *ProviderQuotaService) getCheckInterval() time.Duration {
	if svc.checkInterval > 0 {
		return svc.checkInterval
	}

	return 5 * time.Minute
}

func (svc *ProviderQuotaService) loadQuotaCache(ctx context.Context) {
	records, err := svc.db.ProviderQuotaStatus.Query().All(ctx)
	if err != nil {
		log.Error(ctx, "Failed to load quota cache from DB", log.Cause(err))
		return
	}

	for _, r := range records {
		svc.quotaCache.Store(r.ChannelID, &QuotaChannelStatus{
			ProviderType:    r.ProviderType.String(),
			Status:          r.Status,
			Ready:           r.Ready,
			Limits:          cloneLimits(extractLimitsFromQuotaData(r.QuotaData)),
			LastKnownStatus: lastKnownQuotaStatusFromEntry(r.Status, r.QuotaData),
		})
	}

	log.Debug(ctx, "Loaded quota cache from DB", log.Int("records", len(records)))
}

func (svc *ProviderQuotaService) GetQuotaStatus(ctx context.Context, channelID int) *QuotaChannelStatus {
	val, ok := svc.quotaCache.Load(channelID)
	if !ok {
		return nil
	}

	status, ok := val.(*QuotaChannelStatus)
	if !ok {
		return nil
	}
	if status.ProviderType != "" {
		settings := svc.SystemService.ProviderQuotaCollectionSettingsOrDefault(ctx)
		if !settings.Enabled || !settings.Providers[status.ProviderType] {
			return nil
		}
	}

	return &QuotaChannelStatus{
		ProviderType:    status.ProviderType,
		Status:          status.Status,
		Ready:           status.Ready,
		Limits:          cloneLimits(status.Limits),
		LastKnownStatus: status.LastKnownStatus,
	}
}

// updateQuotaCache records the channel's newest known status (lastKnown: the
// newest non-unknown status, see QuotaChannelStatus.LastKnownStatus)
// alongside the raw status.
func (svc *ProviderQuotaService) updateQuotaCache(channelID int, providerType string, status providerquotastatus.Status, ready bool, lastKnown string, limits []provider_quota.QuotaLimitStatus) {
	svc.quotaCache.Store(channelID, &QuotaChannelStatus{
		ProviderType:    providerType,
		Status:          status,
		Ready:           ready,
		Limits:          cloneLimits(limits),
		LastKnownStatus: lastKnown,
	})
}

// InvalidateChannelQuota removes a channel's persisted and cached quota state.
// Channel provider identity changes invalidate the previous provider's quota
// result, so serialize this with quota checks before removing the record.
func (svc *ProviderQuotaService) InvalidateChannelQuota(ctx context.Context, channelID int) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	return svc.invalidateChannelQuotaLocked(ctx, channelID)
}

// invalidateChannelQuotaLocked removes persisted and cached quota state while
// svc.mu is already held by the quota collection loop.
func (svc *ProviderQuotaService) invalidateChannelQuotaLocked(ctx context.Context, channelID int) error {
	defer svc.quotaCache.Delete(channelID)

	_, err := svc.db.ProviderQuotaStatus.Delete().
		Where(providerquotastatus.ChannelIDEQ(channelID)).
		Exec(schematype.SkipSoftDelete(ctx))
	if err != nil {
		return fmt.Errorf("failed to invalidate provider quota status: %w", err)
	}

	return nil
}

// ManualCheck forces an immediate quota check for all relevant channels.
func (svc *ProviderQuotaService) ManualCheck(ctx context.Context) {
	svc.runQuotaCheckForce(ctx)
}

// ListResets returns the reset capability and available resets for a channel.
// Providers that do not implement Resetter report Supported=false without an
// error so callers can treat resetting as an optional capability.
func (svc *ProviderQuotaService) ListResets(ctx context.Context, channelID int) (provider_quota.ResetList, error) {
	ch, err := svc.db.Channel.Query().Where(channel.IDEQ(channelID)).Only(ctx)
	if err != nil {
		return provider_quota.ResetList{}, fmt.Errorf("failed to load channel: %w", err)
	}

	providerType := svc.getProviderType(ch)
	checker, ok := svc.checkers[providerType]
	if !ok {
		return provider_quota.ResetList{Supported: false}, nil
	}

	resetter, ok := checker.(provider_quota.Resetter)
	if !ok {
		return provider_quota.ResetList{Supported: false}, nil
	}

	if enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType); err != nil {
		return provider_quota.ResetList{}, fmt.Errorf("failed to read provider quota collection settings: %w", err)
	} else if !enabled {
		return provider_quota.ResetList{}, fmt.Errorf("provider quota collection is disabled for %s", providerType)
	}

	if !hasCredentialsForProvider(ch) {
		return provider_quota.ResetList{}, fmt.Errorf("channel has no credentials")
	}

	resets, err := resetter.ListResets(ctx, ch)
	resets.Supported = true
	if err != nil {
		return resets, fmt.Errorf("failed to list %s quota resets: %w", providerType, err)
	}

	return resets, nil
}

// ResetChannelQuotaNow attempts to redeem a provider-managed reset for a channel.
func (svc *ProviderQuotaService) ResetChannelQuotaNow(ctx context.Context, channelID int, resetIDs ...string) error {
	// The quota edge gives saveQuotaStatus the pre-reset previous status for
	// webhook transition detection without an extra query.
	ch, err := svc.db.Channel.Query().Where(channel.IDEQ(channelID)).WithProviderQuotaStatus().Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to load channel: %w", err)
	}

	providerType := svc.getProviderType(ch)
	checker, ok := svc.checkers[providerType]
	if !ok {
		return fmt.Errorf("%w for provider %q", provider_quota.ErrResetUnsupported, providerType)
	}

	resetter, ok := checker.(provider_quota.Resetter)
	if !ok {
		return fmt.Errorf("%w for provider %q", provider_quota.ErrResetUnsupported, providerType)
	}

	if enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType); err != nil {
		return fmt.Errorf("failed to read provider quota collection settings: %w", err)
	} else if !enabled {
		return fmt.Errorf("provider quota collection is disabled for %s", providerType)
	}

	if !hasCredentialsForProvider(ch) {
		return fmt.Errorf("channel has no credentials")
	}

	var resetErr error
	if len(resetIDs) > 0 && resetIDs[0] != "" {
		selective, ok := checker.(provider_quota.SelectiveResetter)
		if !ok {
			return fmt.Errorf("%w: selecting a reset for provider %q", provider_quota.ErrResetUnsupported, providerType)
		}
		resetErr = selective.ResetSelected(ctx, ch, resetIDs[0])
	} else {
		resetErr = resetter.Reset(ctx, ch)
	}
	if resetErr != nil {
		return fmt.Errorf("failed to reset %s quota: %w", providerType, resetErr)
	}

	// Refresh the quota status immediately so the UI reflects the reset.
	// Hold the service mutex to keep the in-memory cache consistent with the DB
	// in case a scheduled quota check is running concurrently.
	svc.mu.Lock()
	// A scheduled check may have saved a fresh status while the provider
	// reset was in flight; reload the edge under the lock so saveQuotaStatus
	// computes the webhook transition against that status, not the
	// pre-reset snapshot loaded before the reset call.
	reloadFailed := false
	reloaded, err := svc.db.Channel.Query().Where(channel.IDEQ(channelID)).WithProviderQuotaStatus().Only(ctx)
	if err != nil {
		// The provider reset already succeeded upstream; a transient DB read
		// failure here must not report the mutation as failed. Proceed with
		// the pre-reset channel snapshot, whose (stale) quota edge still
		// gives saveQuotaStatus a best-effort previous status. The failed
		// reload's refresh compares against the newest known status (cache
		// over stale edge) to avoid replaying an already-emitted transition.
		reloadFailed = true
		log.Error(ctx, "Failed to reload channel after quota reset; refreshing with stale channel",
			log.Int("channel_id", ch.ID),
			log.String("provider", providerType),
			log.Cause(err))
	} else {
		ch = reloaded
	}

	now := time.Now()
	svc.checkChannelQuota(ctx, quotaCheckGroup{
		channels:   []*ent.Channel{ch},
		accountKey: quotaAccountKey(providerType, ch),
		// On a failed reload, the refresh's previous status is the stale
		// pre-reset edge, which would re-emit a transition (e.g. exhausted →
		// available) that already fired. Suppression only drops duplicate
		// events; a genuine transition away from the last-known status is
		// still emitted (see quotaCheckGroup.suppressWebhooks).
		suppressWebhooks: reloadFailed,
	}, now)
	svc.mu.Unlock()

	return nil
}

func (svc *ProviderQuotaService) runQuotaCheckForce(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.runQuotaCheck(ctx, true)
}

func (svc *ProviderQuotaService) runQuotaCheck(ctx context.Context, force bool) {
	ctx = ent.NewContext(ctx, svc.db)
	settings := svc.SystemService.ProviderQuotaCollectionSettingsOrDefault(ctx)
	if !settings.Enabled {
		return
	}

	now := time.Now()
	log.Debug(ctx, "Checking for channels to poll",
		log.Time("now", now),
		log.String("now_formatted", now.Format(time.RFC3339)),
		log.Bool("force", force),
	)

	q := svc.db.Channel.Query().
		Where(
			channel.StatusEQ(channel.StatusEnabled),
			channel.TypeIn(providerQuotaChannelTypes...),
		)

	channelsToCheck, err := q.
		WithProviderQuotaStatus().
		All(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query channels for quota check", log.Cause(err))
		return
	}
	channelsToCheck = lo.Filter(channelsToCheck, func(ch *ent.Channel, _ int) bool {
		providerType := svc.getProviderType(ch)
		return providerType != "" && settings.Providers[providerType]
	})

	if len(channelsToCheck) == 0 {
		log.Debug(ctx, "No channels need quota check at this time")
		return
	}

	log.Info(ctx, "Running quota check",
		log.Int("channels", len(channelsToCheck)),
		log.Bool("force", force),
	)

	channelGroups := svc.groupChannelsByQuotaAccount(channelsToCheck)
	if !force {
		channelGroups = lo.Filter(channelGroups, func(group quotaCheckGroup, _ int) bool {
			return quotaCheckGroupIsDue(group, now)
		})
	}
	if len(channelGroups) == 0 {
		log.Debug(ctx, "No channels need quota check at this time")
		return
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(min(maxConcurrentQuotaChecks, len(channelGroups)))
	for _, group := range channelGroups {
		eg.Go(func() error {
			svc.checkChannelQuota(egCtx, group, now)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		log.Info(ctx, "quota check group interrupted", log.Cause(err))
	}
}

// checkChannelQuota runs under svc.mu, held by both scheduled and manual checks.
// That lets credential failures remove persisted and cached status atomically
// with the rest of the quota collection state.
func (svc *ProviderQuotaService) checkChannelQuota(ctx context.Context, group quotaCheckGroup, now time.Time) {
	ch := group.channels[0]
	providerType := svc.getProviderType(ch)
	if providerType == "" {
		return
	}
	if enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType); err != nil {
		log.Warn(ctx, "failed to read provider quota collection settings",
			log.String("provider", providerType),
			log.Cause(err))
	} else if !enabled {
		return
	}

	if !hasCredentialsForProvider(ch) {
		if err := svc.invalidateChannelQuotaLocked(ctx, ch.ID); err != nil {
			log.Error(ctx, "Failed to invalidate provider quota after credentials disappeared",
				log.Int("channel_id", ch.ID),
				log.String("provider", providerType),
				log.Cause(err))
		}
		log.Debug(ctx, "channel does not support check quota", log.Int("channel_id", ch.ID), log.String("channel_name", ch.Name))
		return
	}

	checker, ok := svc.checkers[providerType]
	if !ok {
		log.Error(ctx, "No checker for provider",
			log.String("provider", providerType),
			log.Int("channel_id", ch.ID))

		return
	}

	// Make quota check request
	quotaData, err := checker.CheckQuota(ctx, ch)
	if err != nil {
		log.Error(ctx, "Quota check failed",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
			log.String("provider", providerType),
			log.Cause(err))

		failures := nextQuotaGroupErrorCount(group.channels, providerType)
		for _, member := range group.channels {
			svc.saveQuotaError(ctx, member, providerType, group.accountKey, err, failures, now)
		}
		return
	}
	quotaData = provider_quota.NormalizeQuotaData(quotaData)

	resetList := provider_quota.ResetList{Supported: false}
	if quotaData.Resets != nil {
		resetList = *quotaData.Resets
	} else if resetter, ok := checker.(provider_quota.Resetter); ok {
		resetList.Supported = true
		resetList, err = resetter.ListResets(ctx, ch)
		resetList.Supported = true
		if err != nil {
			resetList.Error = err.Error()
			log.Warn(ctx, "Failed to list provider quota resets",
				log.Int("channel_id", ch.ID),
				log.String("provider", providerType),
				log.Cause(err))
		}
	}
	quotaData.Resets = &resetList

	for _, member := range group.channels {
		memberQuotaData := quotaData
		memberQuotaData.Limits = cloneLimits(quotaData.Limits)
		memberQuotaData.Balances = cloneBalances(quotaData.Balances)
		// saveQuotaStatus may write _previous_status into RawData, which is a
		// shared map across the account group; clone it so each member's
		// persisted row carries only its own carried status.
		memberQuotaData.RawData = lo.Assign(map[string]any{}, quotaData.RawData)
		svc.fillPeriodQuotas(ctx, member.ID, &memberQuotaData, now)
		svc.saveQuotaStatus(ctx, member, providerType, group.accountKey, memberQuotaData, now, group.suppressWebhooks)

		log.Debug(ctx, "Updated quota status",
			log.Int("channel_id", member.ID),
			log.String("provider", providerType),
			log.String("status", memberQuotaData.Status),
			log.Bool("ready", memberQuotaData.Ready))
	}
}

// suppressWebhooks persists the fresh status while dropping a duplicate
// transition notification: the notification is skipped only when there is no
// genuine transition from the channel's last-known status (see
// quotaCheckGroup.suppressWebhooks).
func (svc *ProviderQuotaService) saveQuotaStatus(
	ctx context.Context,
	ch *ent.Channel,
	providerType string,
	accountKey string,
	quotaData provider_quota.QuotaData,
	now time.Time,
	suppressWebhooks bool,
) {
	nextCheck := now.Add(svc.nextCheckIntervalForStatus(providerquotastatus.Status(quotaData.Status)))
	pt := providerquotastatus.ProviderType(providerType)

	// The previous status is read from the channel's loaded edge (or the
	// in-memory cache, which every save/updateQuotaCache keeps in sync with the
	// DB) before the upsert, so no extra query is needed. It drives webhook
	// transition detection after a successful write. It is only usable when it
	// belongs to the same provider: a channel that switched providers must not
	// report a transition (e.g. the old provider's exhausted as "recovered")
	// against the new provider's first status — same rule saveQuotaError applies.
	//
	// On a failed-reload refresh (suppressWebhooks), the in-memory cache wins
	// over the loaded edge: the edge in the pre-reset snapshot may be stale,
	// while a scheduled check that ran concurrently may have saved the channel's
	// newest status to the cache. Comparing against that (the newest known
	// status) keeps a refresh that merely re-reports it silent; comparing
	// against the stale edge instead would replay the transition the scheduled
	// check already emitted. The cache is kept in sync under the service mutex,
	// which both the scheduled check and the reset refresh hold.
	var previous providerquotastatus.Status
	havePrevious := false
	var rawPrevious string
	var previousFromCache bool
	var previousCache *QuotaChannelStatus
	if suppressWebhooks {
		// Cache first: the freshest last-known status any in-flight scheduled
		// check saved. Fall back to the (stale) edge only when the cache holds
		// no entry for this provider, so at least a best-effort previous status
		// survives an empty cache.
		if cached := svc.GetQuotaStatus(ctx, ch.ID); cached != nil && providerquotastatus.ProviderType(cached.ProviderType) == pt {
			previous = cached.Status
			havePrevious = true
			previousFromCache = true
			previousCache = cached
		} else if existing := ch.Edges.ProviderQuotaStatus; existing != nil && existing.ProviderType == pt {
			previous = existing.Status
			havePrevious = true
		}
	} else if existing := ch.Edges.ProviderQuotaStatus; existing != nil && existing.ProviderType == pt {
		previous = existing.Status
		havePrevious = true
	} else if cached := svc.GetQuotaStatus(ctx, ch.ID); cached != nil && providerquotastatus.ProviderType(cached.ProviderType) == pt {
		previous = cached.Status
		havePrevious = true
		previousFromCache = true
		previousCache = cached
	}
	if havePrevious && !previousFromCache {
		// An edge-sourced previous yields its own persisted carry.
		previousCache = &QuotaChannelStatus{
			Status:          previous,
			LastKnownStatus: lastKnownQuotaStatus(ch, previous),
		}
	}
	if havePrevious {
		rawPrevious = previous.String()
	}

	// _previous_status carries the last known non-unknown status in the
	// persisted quota_data across interstitial `unknown` rows. The write is
	// idempotent, so the carry survives any number of consecutive `unknown`
	// polls; a fresh known status explicitly drops the key so known rows never
	// carry a stale one. The carry is resolved from the source `previous`
	// came from: a cached `unknown` has no persisted quota_data of its own,
	// so its LastKnownStatus (recorded by every save) is monotonically
	// fresher than the stale pre-reset channel edge's carried value.
	newStatus := providerquotastatus.Status(quotaData.Status)
	cacheLastKnown := ""
	if previousCache != nil {
		cacheLastKnown = previousCache.LastKnownStatus
	}
	if havePrevious && newStatus == providerquotastatus.StatusUnknown && cacheLastKnown != "" {
		if quotaData.RawData == nil {
			quotaData.RawData = map[string]any{}
		}
		quotaData.RawData["_previous_status"] = cacheLastKnown
	} else if quotaData.RawData != nil {
		delete(quotaData.RawData, "_previous_status")
	}

	create := svc.db.ProviderQuotaStatus.Create().
		SetChannelID(ch.ID).
		SetProviderType(pt).
		SetAccountKey(accountKey).
		SetStatus(providerquotastatus.Status(quotaData.Status)).
		SetQuotaData(svc.mergeLimitsIntoQuotaData(quotaData)).
		SetNextCheckAt(nextCheck)

	// Only set next_reset_at if it exists (it's optional in schema)
	if quotaData.NextResetAt != nil {
		create.SetNextResetAt(*quotaData.NextResetAt)
	}

	// Set ready based on status
	create.SetReady(quotaData.Ready)

	upsert := create.
		OnConflict(
			sql.ConflictColumns("channel_id"),
		).
		UpdateNewValues()
	if quotaData.NextResetAt == nil {
		upsert.ClearNextResetAt()
	}

	err := upsert.Exec(ctx)
	if err != nil {
		log.Error(ctx, "Failed to save quota status",
			log.Int("channel_id", ch.ID),
			log.Cause(err))
		return
	}

	svc.updateQuotaCache(ch.ID, providerType, newStatus, quotaData.Ready, lastKnownQuotaStatusFromEntry(newStatus, quotaData.RawData), quotaData.Limits)

	// The transition is selected against the last known status of the source
	// `previous` came from: the cache's monotonically fresh LastKnownStatus
	// when the cache won, the loaded edge's own persisted carry otherwise —
	// never the stale pre-reset channel edge's carry, so a failed-reload
	// refresh compares against the freshest carried status.
	freshLastKnown := ""
	if previousCache != nil {
		freshLastKnown = previousCache.LastKnownStatus
	}
	if havePrevious && (!suppressWebhooks || quotaStatusTransitionEvent(freshLastKnown, newStatus) != "") {
		svc.notifyQuotaStatusChange(ctx, ch, rawPrevious, freshLastKnown, quotaData)
	}
}

// quotaStatusTransitionEvent selects the webhook event a status transition
// would fire, for callers that pre-gate (and may suppress) the notification
// before reaching notifyQuotaStatusChange. It mirrors that function's event
// selection exactly.
func quotaStatusTransitionEvent(lastKnown string, newStatus providerquotastatus.Status) string {
	if lastKnown == "" || lastKnown == newStatus.String() {
		return ""
	}

	switch {
	case newStatus == providerquotastatus.StatusWarning &&
		(lastKnown == providerquotastatus.StatusAvailable.String() ||
			lastKnown == providerquotastatus.StatusWarning.String()):
		return EventProviderQuotaWarning
	case newStatus == providerquotastatus.StatusExhausted &&
		(lastKnown == providerquotastatus.StatusAvailable.String() ||
			lastKnown == providerquotastatus.StatusWarning.String()):
		return EventProviderQuotaExhausted
	case newStatus == providerquotastatus.StatusAvailable &&
		(lastKnown == providerquotastatus.StatusWarning.String() ||
			lastKnown == providerquotastatus.StatusExhausted.String()):
		return EventProviderQuotaRecovered
	}
	return ""
}

// lastKnownQuotaStatus returns the channel's last known non-unknown quota
// status as a string: the status last observed (edge or cache) when it is not
// `unknown`, otherwise the `_previous_status` carried in the persisted
// quota_data of the loaded edge, and finally "" for a channel that never
// produced a status. This is what makes `warning → unknown (check failed) →
// available` emit provider.quota.recovered, which a plain status comparison
// would drop.
// lastKnownQuotaStatusFromEntry resolves the last known non-unknown status
// for one persisted quota row: a status that is not `unknown` is itself,
// otherwise the `_previous_status` carried in its persisted quota_data
// (empty when none).
func lastKnownQuotaStatusFromEntry(status providerquotastatus.Status, quotaData map[string]any) string {
	if status != providerquotastatus.StatusUnknown {
		return status.String()
	}

	if raw, ok := quotaData["_previous_status"]; ok {
		if s, ok := raw.(string); ok {
			return s
		}
	}

	return ""
}

func lastKnownQuotaStatus(ch *ent.Channel, lastSeen providerquotastatus.Status) string {
	if lastSeen != providerquotastatus.StatusUnknown {
		return lastSeen.String()
	}

	if ch.Edges.ProviderQuotaStatus != nil && ch.Edges.ProviderQuotaStatus.QuotaData != nil {
		if raw, ok := ch.Edges.ProviderQuotaStatus.QuotaData["_previous_status"]; ok {
			if s, ok := raw.(string); ok {
				return s
			}
		}
	}

	return ""
}

// notifyQuotaStatusChange emits the webhook event for a quota status
// transition, if the transition is one that fires an event:
//
//   - available|warning → warning   = provider.quota.warning
//   - available|warning → exhausted = provider.quota.exhausted (never also a warning)
//   - warning|exhausted → available = provider.quota.recovered
//
// Same-status saves, the first-ever status, and transitions involving
// `unknown` as the fresh status are silent.
func (svc *ProviderQuotaService) notifyQuotaStatusChange(
	ctx context.Context,
	ch *ent.Channel,
	rawPrevious string,
	previousLastKnown string,
	quotaData provider_quota.QuotaData,
) {
	// previousLastKnown == "" means the channel has no previous status of any
	// kind (first-ever detection): nothing fires.
	newStatus := providerquotastatus.Status(quotaData.Status)
	eventName := quotaStatusTransitionEvent(previousLastKnown, newStatus)
	if eventName == "" {
		return
	}

	// rawPrevious is the raw pre-save status (edge or cache), captured in
	// saveQuotaStatus before the upsert and cache update, so it is not the
	// fresh status on the edge-less ResetChannelQuotaNow path.

	// Display-only limits (e.g. NeuralWatt's daily free energy promo) are
	// excluded from the max so a spent promo does not report usage_ratio: 1.0
	// on later monthly-driven events.
	usageRatio := -1.0
	for _, limit := range quotaData.Limits {
		if provider_quota.IsDisplayOnlyLimit(limit) {
			continue
		}
		if usageRatio < 0 || limit.UsageRatio > usageRatio {
			usageRatio = limit.UsageRatio
		}
	}

	var nextResetAt string
	if quotaData.NextResetAt != nil {
		nextResetAt = quotaData.NextResetAt.Format(time.RFC3339)
	}

	// Providers report several windows of the same limit type (e.g. Synthetic
	// emits both a 5h and a weekly token window), so each window gets its own
	// line; identical duplicates are dropped by full-line identity. Multi-account
	// channels append the account label so the same window from two accounts
	// stays distinct; single-account limits keep the plain line byte-identical
	// to the account-less format.
	limits := make([]string, 0, len(quotaData.Limits))
	seenLines := make(map[string]struct{}, len(quotaData.Limits))
	for _, limit := range quotaData.Limits {
		line := fmt.Sprintf(
			"%s (%s) %.2f %s", limit.Type, limit.Window, limit.UsageRatio, limit.Status,
		)
		if limit.Account != "" {
			line = fmt.Sprintf("%s [account: %s]", line, limit.Account)
		}
		if _, ok := seenLines[line]; ok {
			continue
		}
		seenLines[line] = struct{}{}
		limits = append(limits, line)
	}

	event := QuotaStatusChangedEvent{
		ChannelID:       ch.ID,
		ChannelName:     ch.Name,
		ChannelProvider: ch.Type.String(),
		ChannelBaseURL:  ch.BaseURL,
		ChannelStatus:   ch.Status.String(),
		PreviousStatus:  rawPrevious,
		NewStatus:       newStatus.String(),
		UsageRatio:      usageRatio,
		NextResetAt:     nextResetAt,
		Limits:          limits,
		OccurredAt:      time.Now(),
	}

	notifyCtx := context.WithoutCancel(ctx)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(notifyCtx, "provider quota webhook notification panicked", log.Any("panic", r))
			}
		}()

		switch eventName {
		case EventProviderQuotaWarning:
			svc.WebhookNotifier.NotifyProviderQuotaWarning(notifyCtx, event)
		case EventProviderQuotaExhausted:
			svc.WebhookNotifier.NotifyProviderQuotaExhausted(notifyCtx, event)
		case EventProviderQuotaRecovered:
			svc.WebhookNotifier.NotifyProviderQuotaRecovered(notifyCtx, event)
		}
	}()
}

func (svc *ProviderQuotaService) saveQuotaError(
	ctx context.Context,
	ch *ent.Channel,
	providerType string,
	accountKey string,
	quotaErr error,
	failures int,
	now time.Time,
) {
	pt := providerquotastatus.ProviderType(providerType)
	nextCheck := now.Add(quotaErrorBackoff(svc.getCheckInterval(), failures))
	errorCode := quotaErrorCode(quotaErr)

	if ch.Edges.ProviderQuotaStatus != nil {
		existing := ch.Edges.ProviderQuotaStatus
		providerChanged := existing.ProviderType != pt
		invalidCredentials := errors.Is(quotaErr, provider_quota.ErrInvalidCredentials)
		if providerChanged || invalidCredentials {
			nextCheck := now.Add(quotaErrorBackoff(svc.getCheckInterval(), 1))
			quotaData := map[string]any{
				"error_code":  errorCode,
				"error_count": 1,
			}

			// A provider change makes the previous status and limits
			// untrustworthy, so the row carries only the error. For invalid
			// credentials alone the
			// provider identity is unchanged and the last known status is
			// re-usable after re-authentication, so preserve the carried
			// _previous_status and let webhook recovery detection keep
			// working.
			lastKnown := ""
			if !providerChanged {
				lastKnown = existing.Status.String()
				if lastKnown == providerquotastatus.StatusUnknown.String() {
					lastKnown, _ = existing.QuotaData["_previous_status"].(string)
				}
				if lastKnown == providerquotastatus.StatusUnknown.String() || lastKnown == "" {
					lastKnown = ""
				}
				if lastKnown != "" {
					quotaData["_previous_status"] = lastKnown
				}
			}

			err := svc.db.ProviderQuotaStatus.UpdateOne(existing).
				SetProviderType(pt).
				SetAccountKey(accountKey).
				SetStatus(providerquotastatus.StatusUnknown).
				SetReady(false).
				SetQuotaData(quotaData).
				ClearNextResetAt().
				SetNextCheckAt(nextCheck).
				Exec(ctx)
			if err != nil {
				log.Error(ctx, "Failed to reset quota status after quota check error",
					log.Int("channel_id", ch.ID),
					log.String("previous_provider", existing.ProviderType.String()),
					log.String("provider", providerType),
					log.Cause(err))
				return
			}

			svc.updateQuotaCache(ch.ID, providerType, providerquotastatus.StatusUnknown, false, lastKnown, nil)
			return
		}

		existingData := existing.QuotaData
		if existingData == nil {
			existingData = map[string]any{}
		}

		merged := lo.Assign(existingData, map[string]any{
			"error_code":  errorCode,
			"error_count": failures,
		})
		delete(merged, "error")

		err := svc.db.ProviderQuotaStatus.UpdateOne(existing).
			SetAccountKey(accountKey).
			SetQuotaData(merged).
			SetNextCheckAt(nextCheck).
			Exec(ctx)
		if err != nil {
			log.Error(ctx, "Failed to save quota error",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			return
		}

		existingLimits := extractLimitsFromQuotaData(existing.QuotaData)
		svc.updateQuotaCache(ch.ID, providerType, existing.Status, existing.Ready, lastKnownQuotaStatusFromEntry(existing.Status, merged), existingLimits)

		return
	}

	err := svc.db.ProviderQuotaStatus.Create().
		SetChannelID(ch.ID).
		SetProviderType(pt).
		SetAccountKey(accountKey).
		SetStatus(providerquotastatus.StatusUnknown).
		SetReady(false).
		SetQuotaData(map[string]any{
			"error_code":  errorCode,
			"error_count": failures,
		}).
		SetNextCheckAt(nextCheck).
		Exec(ctx)
	if err != nil {
		log.Error(ctx, "Failed to save quota status",
			log.Int("channel_id", ch.ID),
			log.Cause(err))
		return
	}

	svc.updateQuotaCache(ch.ID, providerType, providerquotastatus.StatusUnknown, false, "", nil)
}

// getProviderType maps a channel to the provider quota key used by the checker
// registry and collection settings.
func (svc *ProviderQuotaService) getProviderType(ch *ent.Channel) string {
	switch ch.Type { //nolint:exhaustive
	case channel.TypeClaudecode:
		return "claudecode"
	case channel.TypeCodex:
		return "codex"
	case channel.TypeAntigravity:
		return "antigravity"
	case channel.TypeXaiSubscription:
		return "xai_subscription"
	case channel.TypeGithubCopilot:
		return "github_copilot"
	case channel.TypeNanogpt, channel.TypeNanogptResponses:
		return "nanogpt"
	case channel.TypeZenmux, channel.TypeZenmuxResponses, channel.TypeZenmuxAnthropic, channel.TypeZenmuxGemini, channel.TypeZenmuxVideo:
		return "zenmux"
	case channel.TypeCline:
		return "cline"
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		return provider_quota.DetectProviderFromURL(ch.BaseURL)
	case channel.TypeOpencodeGo, channel.TypeOpencodeGoAnthropic:
		return "opencode_go"
	case channel.TypeMoonshotCoding:
		return "kimi_code"
	case channel.TypeMinimax, channel.TypeMinimaxAnthropic:
		return "minimax"
	case channel.TypeZhipu, channel.TypeZhipuAnthropic:
		return "zhipu"
	case channel.TypeZai, channel.TypeZaiAnthropic:
		return "zai"
	case channel.TypeDeepseek, channel.TypeDeepseekAnthropic:
		return "deepseek"
	case channel.TypeCommandcode, channel.TypeCommandcodeAnthropic:
		return "commandcode"
	case channel.TypeOllama, channel.TypeOllamaAnthropic:
		return "ollama"
	default:
		return ""
	}
}

func hasCredentialsForProvider(ch *ent.Channel) bool {
	switch ch.Type { //nolint:exhaustive // Only ZenMux uses the separate management credential.
	case channel.TypeZenmux, channel.TypeZenmuxResponses, channel.TypeZenmuxAnthropic, channel.TypeZenmuxGemini, channel.TypeZenmuxVideo:
		return strings.TrimSpace(ch.Credentials.ManagementAPIKey) != ""
	default:
	}

	if ch.Type == channel.TypeOpenai || ch.Type == channel.TypeOpenaiResponses {
		providerType := provider_quota.DetectProviderFromURL(ch.BaseURL)
		if _, ok := provider_quota.URLDetectedProviders()[providerType]; ok {
			return strings.TrimSpace(ch.Credentials.APIKey) != "" || len(ch.Credentials.APIKeys) > 0
		}
	}

	if ch.Type == channel.TypeCodex || ch.Type == channel.TypeClaudecode || ch.Type == channel.TypeXaiSubscription {
		return ch.Credentials.OAuth != nil || isOAuthJSON(ch.Credentials.APIKey)
	}

	if ch.Type == channel.TypeCline {
		if strings.TrimSpace(ch.Credentials.APIKey) != "" {
			return true
		}
		for _, apiKey := range ch.Credentials.APIKeys {
			if strings.TrimSpace(apiKey) != "" {
				return true
			}
		}
		return false
	}

	if isCommandCodeChannelType(ch.Type) {
		// Command Code quota collection authenticates with the account API key
		// (/alpha/billing/*), or with the Studio session cookie as a fallback.
		return provider_quota.HasCommandCodeQuotaCredentials(ch)
	}

	if ch.Type == channel.TypeOllama || ch.Type == channel.TypeOllamaAnthropic {
		// Ollama Cloud quota collection authenticates with the account API key
		// (/api/balance), or with the session cookie as a fallback.
		return provider_quota.HasOllamaQuotaCredentials(ch)
	}

	return ch.Credentials.OAuth != nil || isOAuthJSON(ch.Credentials.APIKey) ||
		strings.TrimSpace(ch.Credentials.APIKey) != "" || len(ch.Credentials.APIKeys) > 0
}

// mergeLimitsIntoQuotaData merges normalized limits, balances and resets into
// the provider-specific payload persisted on the channel quota status.
func (svc *ProviderQuotaService) mergeLimitsIntoQuotaData(quotaData provider_quota.QuotaData) map[string]any {
	data := lo.Assign(map[string]any{}, quotaData.RawData)
	if quotaData.Resets != nil {
		data["_resets"] = quotaData.Resets
	}

	if len(quotaData.Limits) > 0 {
		limitMaps := make([]map[string]any, 0, len(quotaData.Limits))
		for _, l := range quotaData.Limits {
			m := map[string]any{
				"type":       string(l.Type),
				"status":     l.Status,
				"usageRatio": l.UsageRatio,
				"ready":      l.Ready,
			}
			if l.NextResetAt != nil {
				m["nextResetAt"] = l.NextResetAt.Format(time.RFC3339)
			}
			if l.Window != "" {
				m["window"] = l.Window
			}
			if l.Account != "" {
				m["account"] = l.Account
			}
			if l.AvailabilityGroup != "" {
				m["availabilityGroup"] = l.AvailabilityGroup
			}
			if l.PeriodStart != nil {
				m["periodStart"] = l.PeriodStart.Format(time.RFC3339)
			}
			if l.PeriodCost != nil {
				m["periodCost"] = *l.PeriodCost
			}
			if l.PeriodQuota != nil {
				m["periodQuota"] = *l.PeriodQuota
			}
			limitMaps = append(limitMaps, m)
		}
		data["_limits"] = limitMaps
	}

	if len(quotaData.Balances) > 0 {
		balanceMaps := make([]map[string]any, 0, len(quotaData.Balances))
		for _, b := range quotaData.Balances {
			m := map[string]any{
				"total": b.Total,
			}
			if b.Currency != "" {
				m["currency"] = b.Currency
			}
			if b.Account != "" {
				m["account"] = b.Account
			}
			if b.Granted != nil {
				m["granted"] = *b.Granted
			}
			if b.ToppedUp != nil {
				m["topped_up"] = *b.ToppedUp
			}
			balanceMaps = append(balanceMaps, m)
		}
		data["_balances"] = balanceMaps
	}

	return data
}

func extractLimitsFromQuotaData(data map[string]any) []provider_quota.QuotaLimitStatus {
	rawLimits, ok := data["_limits"]
	if !ok {
		return nil
	}

	// Handle both []map[string]any (from mergeLimitsIntoQuotaData) and []any (from JSON unmarshaling)
	var limitMaps []map[string]any
	if directMaps, ok := rawLimits.([]map[string]any); ok {
		limitMaps = directMaps
	} else if anySlice, ok := rawLimits.([]any); ok {
		limitMaps = make([]map[string]any, 0, len(anySlice))
		for _, raw := range anySlice {
			if m, ok := raw.(map[string]any); ok {
				limitMaps = append(limitMaps, m)
			}
		}
	} else {
		return nil
	}

	var limits []provider_quota.QuotaLimitStatus

	for _, m := range limitMaps {
		ls := provider_quota.QuotaLimitStatus{}

		if t, ok := m["type"].(string); ok {
			ls.Type = provider_quota.QuotaLimitType(t)
		}

		if s, ok := m["status"].(string); ok {
			ls.Status = s
		}

		if u, ok := m["usageRatio"].(float64); ok {
			ls.UsageRatio = u
		}

		if r, ok := m["ready"].(bool); ok {
			ls.Ready = r
		}

		if ts, ok := m["nextResetAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				ls.NextResetAt = &t
			}
		}

		if w, ok := m["window"].(string); ok {
			ls.Window = w
		}

		if a, ok := m["account"].(string); ok {
			ls.Account = a
		}

		if g, ok := m["availabilityGroup"].(string); ok {
			ls.AvailabilityGroup = g
		}

		if ts, ok := m["periodStart"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				ls.PeriodStart = &t
			}
		}

		if c, ok := m["periodCost"].(float64); ok {
			ls.PeriodCost = &c
		}

		if q, ok := m["periodQuota"].(float64); ok {
			ls.PeriodQuota = &q
		}

		limits = append(limits, ls)
	}

	return limits
}
