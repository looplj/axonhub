package biz

import (
	"context"
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

type ProviderQuotaServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
}

type ProviderQuotaService struct {
	*AbstractService

	SystemService *SystemService
	Executor      executors.ScheduledExecutor

	// Registry
	parsers  map[string]provider_quota.QuotaParser
	checkers map[string]provider_quota.QuotaChecker

	mu sync.Mutex
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
	ctx := context.Background()

	svc := &ProviderQuotaService{
		AbstractService: &AbstractService{db: params.Ent},
		SystemService:   params.SystemService,
		Executor:        executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
		parsers:         make(map[string]provider_quota.QuotaParser),
		checkers:        make(map[string]provider_quota.QuotaChecker),
	}

	// Register providers
	svc.registerClaudeCodeSupport()
	svc.registerCodexSupport()

	log.Info(ctx, "Starting ProviderQuotaService with cron schedule: every 1 minute")

	// Start polling
	if err := svc.Start(ctx); err != nil {
		log.Error(ctx, "Failed to start ProviderQuotaService", log.Cause(err))
		panic(err)
	}

	log.Info(ctx, "ProviderQuotaService started successfully")

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
	// Run every minute
	_, err := svc.Executor.ScheduleFuncAtCronRate(
		svc.runQuotaCheck,
		executors.CRONRule{Expr: "* * * * *"},
	)
	return err
}

func (svc *ProviderQuotaService) Stop(ctx context.Context) error {
	return svc.Executor.Shutdown(ctx)
}

func (svc *ProviderQuotaService) runQuotaCheck(ctx context.Context) {
	ctx = ent.NewContext(ctx, svc.db)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	now := time.Now()
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

		// Store error status with error message in quota_data
		errorData := map[string]interface{}{
			"error": err.Error(),
		}
		svc.saveQuotaStatus(ctx, ch.ID, providerType, "error", errorData, now)
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

		errorData := map[string]interface{}{
			"error": err.Error(),
		}
		svc.saveQuotaStatus(ctx, ch.ID, providerType, "parse_error", errorData, now)
		return
	}

	// Validate parsed data - check if parser returned empty data
	if quotaData.Status == "" && quotaData.RawData == nil {
		log.Warn(ctx, "Parser returned empty quota data (no rate limit headers found)",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
			log.String("provider", providerType))

		// Store a more informative status
		noDataMap := map[string]interface{}{
			"message": "No rate limit headers found in response",
		}
		svc.saveQuotaStatus(ctx, ch.ID, providerType, "no_data", noDataMap, now)
		return
	}

	// Save quota status
	svc.saveQuotaStatus(ctx, ch.ID, providerType, quotaData.Status, quotaData.RawData, now)

	log.Debug(ctx, "Updated quota status",
		log.Int("channel_id", ch.ID),
		log.String("provider", providerType),
		log.String("status", quotaData.Status))
}

func (svc *ProviderQuotaService) saveQuotaStatus(
	ctx context.Context,
	channelID int,
	providerType string,
	status string,
	quotaData map[string]interface{},
	now time.Time,
) {
	nextCheck := now.Add(20 * time.Minute)

	err := svc.db.ProviderQuotaStatus.Create().
		SetChannelID(channelID).
		SetProviderType(providerquotastatus.ProviderType(providerType)).
		SetStatus(status).
		SetQuotaData(quotaData).
		SetNextCheckAt(nextCheck).
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
