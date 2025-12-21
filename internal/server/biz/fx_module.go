package biz

import (
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
)
