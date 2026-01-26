package biz

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("biz",
	fx.Provide(NewSystemService),
	fx.Provide(NewAuthService),
	fx.Provide(NewChannelService),
	fx.Provide(NewRequestService),
	fx.Provide(NewUsageLogService),
	fx.Provide(NewUserService),
	fx.Provide(NewAPIKeyService),
	fx.Provide(NewProjectService),
	fx.Provide(NewRoleService),
	fx.Provide(NewThreadService),
	fx.Provide(NewTraceService),
	fx.Provide(NewDataStorageService),
	fx.Provide(NewChannelOverrideTemplateService),
	fx.Provide(NewModelService),
	fx.Provide(NewBackupService),
	fx.Provide(NewChannelProbeService),
	fx.Provide(NewPromptService),
	fx.Provide(NewQuotaService),
	fx.Provide(NewProviderQuotaService),
	fx.Invoke(func(lc fx.Lifecycle, svc *ProviderQuotaService) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return svc.Start(ctx)
			},
			OnStop: func(ctx context.Context) error {
				return svc.Stop(ctx)
			},
		})
	}),
)
