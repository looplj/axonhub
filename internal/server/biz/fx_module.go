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
)
