package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

// HOW TO ADD A NEW PROVIDER QUOTA CHECKER
// ========================================
//
// 1. Create the checker and parser in internal/server/biz/provider_quota/
//
//    Implement the QuotaChecker interface:
//      - CheckQuota(ctx, ch) -> makes the actual API request to get quota info
//      - Returns (headers, body, error)
//
//    Implement the QuotaParser interface:
//      - ParseResponse(headers, body) -> parses the response into normalized QuotaData
//      - Returns QuotaData with:
//        * Status: "available", "warning", "exhausted", or "unknown"
//        * ready: true for available/warning, false for exhausted/unknown
//        * NextResetAt: optional timestamp of next quota reset
//        * RawData: provider-specific data (will be stored in JSON format)
//
// 2. Add the provider type to the database schema
//
//    In internal/ent/schema/channel.go:
//      - Add new value to the channel.Type enum (e.g., "myprovider")
//
//    In internal/ent/schema/provider_quota_status.go:
//      - Add new value to the provider_type enum (e.g., "myprovider")
//
// 3. Register the provider in ProviderQuotaService
//
//    a. Create a registration function (e.g., registerMyProviderSupport())
//    b. Add it to NewProviderQuotaService()
//    c. Update getProviderType() to map channel.TypeMyprovider -> "myprovider"
//    d. Update runQuotaCheck() to include channel.TypeMyprovider in TypeIn filter
//
//    Example:
//
//      func (svc *ProviderQuotaService) registerMyProviderSupport() {
//        svc.parsers["myprovider"] = &provider_quota.MyProviderQuotaParser{}
//        svc.checkers["myprovider"] = provider_quota.NewMyProviderQuotaChecker()
//      }
//
// 4. regenerate Ent schema
//
//    make generate
//
// 5. Implement the frontend display (optional)
//
//    Add provider-specific display logic in frontend/src/components/quota-badges.tsx:
//      - Update QuotaData type to include provider-specific fields
//      - Add display logic for the provider type in QuotaRow component
//
// EXAMPLE: CLAUDE CODE PROVIDER
// =============================
//
// Checker: internal/server/biz/provider_quota/claudecode_checker.go
//   - Makes minimal request to Claude Code API
//   - Extracts rate limit headers (anthropic-ratelimit-unified-status, etc.)
//
// Parser: internal/server/biz/provider_quota/claudecode_parser.go
//   - Parses rate limit headers
//   - Normalizes status (allowed -> available, throttled -> exhausted)
//   - Detects warning state (utilization >= 80%)
//   - Maps representative claim to reset time
//
// EXAMPLE: CODEX PROVIDER
// ======================
//
// Checker: internal/server/biz/provider_quota/codex_checker.go
//   - Makes request to ChatGPT usage endpoint (/backend-api/wham/usage)
//   - Extracts JSON response with rate limit info
//
// Parser: internal/server/biz/provider_quota/codex_parser.go
//   - Parses JSON response (plan_type, rate_limit)
//   - Normalizes status based on limit_reached and allowed flags
//   - Detects warning state (primary_window.used_percent >= 80)
//

type ProviderQuotaServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
	CheckInterval time.Duration `name:"provider_quota_check_interval" optional:"true"`
}

type ProviderQuotaService struct {
	*AbstractService

	SystemService *SystemService
	Executor      executors.ScheduledExecutor
	checkInterval time.Duration

	// Registry
	parsers  map[string]provider_quota.QuotaParser
	checkers map[string]provider_quota.QuotaChecker

	mu sync.Mutex
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
	svc := &ProviderQuotaService{
		AbstractService: &AbstractService{db: params.Ent},
		SystemService:   params.SystemService,
		Executor:        executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
		parsers:         make(map[string]provider_quota.QuotaParser),
		checkers:        make(map[string]provider_quota.QuotaChecker),
		checkInterval:   params.CheckInterval,
	}

	svc.registerClaudeCodeSupport()
	svc.registerCodexSupport()

	return svc
}

func (svc *ProviderQuotaService) registerClaudeCodeSupport() {
	svc.parsers["claudecode"] = &provider_quota.ClaudeCodeQuotaParser{}
	svc.checkers["claudecode"] = provider_quota.NewClaudeCodeQuotaChecker()
}

func (svc *ProviderQuotaService) registerCodexSupport() {
	svc.parsers["codex"] = &provider_quota.CodexQuotaParser{}
	svc.checkers["codex"] = provider_quota.NewCodexQuotaChecker()
}

func (svc *ProviderQuotaService) Start(ctx context.Context) error {
	// Convert check interval to cron expression
	// For example, 20m -> */20 * * * *, 1h -> 0 * * * *, 30m -> */30 * * * *
	cronExpr := svc.intervalToCronExpr(svc.getCheckInterval())

	_, err := svc.Executor.ScheduleFuncAtCronRate(
		svc.runQuotaCheck,
		executors.CRONRule{Expr: cronExpr},
	)
	return err
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
	var rounded int
	for _, si := range supportedIntervals {
		if minutes <= si {
			rounded = si
			break
		}
	}
	if rounded == 0 {
		rounded = 60
	}

	log.Warn(context.Background(), "Quota check interval does not divide evenly into 60 minutes, rounding to nearest supported interval",
		log.Int("requested_minutes", minutes),
		log.Int("rounded_minutes", rounded))

	return fmt.Sprintf("*/%d * * * *", rounded)
}

func (svc *ProviderQuotaService) getCheckInterval() time.Duration {
	if svc.checkInterval > 0 {
		return svc.checkInterval
	}
	return 20 * time.Minute
}

func (svc *ProviderQuotaService) Stop(ctx context.Context) error {
	return svc.Executor.Shutdown(ctx)
}

// ManualCheck forces an immediate quota check for all relevant channels
func (svc *ProviderQuotaService) ManualCheck(ctx context.Context) {
	svc.runQuotaCheck(ctx)
}

func (svc *ProviderQuotaService) runQuotaCheck(ctx context.Context) {
	ctx = ent.NewContext(ctx, svc.db)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	now := time.Now()
	log.Debug(ctx, "Checking for channels to poll",
		log.Time("now", now),
		log.String("now_formatted", now.Format(time.RFC3339)))

	// Find all enabled channels of supported types first
	allEnabledChannels, err := svc.db.Channel.Query().
		Where(
			channel.StatusEQ(channel.StatusEnabled),
			channel.TypeIn(channel.TypeClaudecode, channel.TypeCodex),
		).
		WithProviderQuotaStatus().
		All(ctx)

	if err != nil {
		log.Error(ctx, "Failed to query enabled channels", log.Cause(err))
		return
	}

	log.Debug(ctx, "Found enabled channels",
		log.Int("total", len(allEnabledChannels)))

	for _, ch := range allEnabledChannels {
		if ch.Edges.ProviderQuotaStatus != nil {
			log.Debug(ctx, "Channel has quota status",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", ch.Name),
				log.Time("next_check_at", ch.Edges.ProviderQuotaStatus.NextCheckAt),
				log.Bool("should_check", ch.Edges.ProviderQuotaStatus.NextCheckAt.Before(now) || ch.Edges.ProviderQuotaStatus.NextCheckAt.Equal(now)))
		} else {
			log.Debug(ctx, "Channel has no quota status",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", ch.Name))
		}
	}

	// Find channels needing quota check:
	// 1. Enabled channels with supported types
	// 2. No quota status OR next_check_at <= now
	channelsToCheck, err := svc.db.Channel.Query().
		Where(
			channel.StatusEQ(channel.StatusEnabled),
			channel.TypeIn(channel.TypeClaudecode, channel.TypeCodex),
			channel.Or(
				channel.Not(channel.HasProviderQuotaStatus()),
				channel.HasProviderQuotaStatusWith(
					providerquotastatus.NextCheckAtLTE(now),
				),
			),
		).
		WithProviderQuotaStatus().
		All(ctx)

	if err != nil {
		log.Error(ctx, "Failed to query channels for quota check", log.Cause(err))
		return
	}

	if len(channelsToCheck) == 0 {
		log.Debug(ctx, "No channels need quota check at this time")
		return
	}

	log.Info(ctx, "Running quota check", log.Int("channels", len(channelsToCheck)))

	for _, ch := range channelsToCheck {
		svc.checkChannelQuota(ctx, ch, now)
	}
}

func (svc *ProviderQuotaService) checkChannelQuota(ctx context.Context, ch *ent.Channel, now time.Time) {
	providerType := svc.getProviderType(ch)
	if providerType == "" {
		return
	}

	checker, ok := svc.checkers[providerType]
	if !ok {
		log.Error(ctx, "No checker for provider",
			log.String("provider", providerType),
			log.Int("channel_id", ch.ID))
		return
	}

	parser, ok := svc.parsers[providerType]
	if !ok {
		log.Error(ctx, "No parser for provider",
			log.String("provider", providerType),
			log.Int("channel_id", ch.ID))
		return
	}

	// Make quota check request
	headers, body, err := checker.CheckQuota(ctx, ch)
	if err != nil {
		log.Error(ctx, "Quota check failed",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
			log.String("provider", providerType),
			log.Cause(err))

		// Store error status
		svc.saveQuotaStatus(ctx, ch.ID, providerType, provider_quota.QuotaData{
			Status: "exhausted",
			RawData: map[string]interface{}{
				"error": err.Error(),
			},
			Ready: false,
		}, now)
		return
	}

	// Parse quota data
	quotaData, err := parser.ParseResponse(headers, body)
	if err != nil {
		log.Error(ctx, "Failed to parse quota response",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
			log.String("provider", providerType),
			log.Cause(err))

		svc.saveQuotaStatus(ctx, ch.ID, providerType, provider_quota.QuotaData{
			Status: "unknown",
			RawData: map[string]interface{}{
				"error": err.Error(),
			},
			Ready: false,
		}, now)
		return
	}

	// Save quota status
	svc.saveQuotaStatus(ctx, ch.ID, providerType, quotaData, now)

	log.Debug(ctx, "Updated quota status",
		log.Int("channel_id", ch.ID),
		log.String("provider", providerType),
		log.String("status", quotaData.Status),
		log.Bool("ready", quotaData.Ready))
}

func (svc *ProviderQuotaService) saveQuotaStatus(
	ctx context.Context,
	channelID int,
	providerType string,
	quotaData provider_quota.QuotaData,
	now time.Time,
) {
	nextCheck := now.Add(svc.getCheckInterval())
	pt := providerquotastatus.ProviderType(providerType)

	create := svc.db.ProviderQuotaStatus.Create().
		SetChannelID(channelID).
		SetProviderType(pt).
		SetStatus(providerquotastatus.Status(quotaData.Status)).
		SetQuotaData(quotaData.RawData).
		SetNextCheckAt(nextCheck)

	// Only set next_reset_at if it exists (it's optional in schema)
	if quotaData.NextResetAt != nil {
		create.SetNextResetAt(*quotaData.NextResetAt)
	}

	// Set ready based on status
	create.SetReady(quotaData.Ready)

	err := create.
		OnConflict(
			sql.ConflictColumns("channel_id"),
		).
		UpdateNewValues().
		Exec(ctx)

	if err != nil {
		log.Error(ctx, "Failed to save quota status",
			log.Int("channel_id", channelID),
			log.Cause(err))
	}
}

func (svc *ProviderQuotaService) getProviderType(ch *ent.Channel) string {
	switch ch.Type {
	case channel.TypeClaudecode:
		return "claudecode"
	case channel.TypeCodex:
		return "codex"
	default:
		return ""
	}
}
