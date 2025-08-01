package api

import (
	"go.uber.org/fx"
)

var Module = fx.Module("api",
	fx.Provide(NewOpenAIHandlers),
	fx.Provide(NewAnthropicHandlers),
	fx.Provide(NewAiSDKHandlers),
	fx.Provide(NewSystemHandlers),
	fx.Provide(NewAuthHandlers),
	fx.Invoke(initLogger),
)
