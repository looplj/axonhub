package api

import (
	"go.uber.org/fx"
)

var Module = fx.Module("api",
	fx.Provide(NewOpenAIHandlers),
	fx.Provide(NewAnthropicHandlers),
	fx.Provide(NewAiSDKHandlers),
	fx.Provide(NewPlaygroundHandlers),
	fx.Provide(NewSystemHandlers),
	fx.Provide(NewAuthHandlers),
	fx.Invoke(initLogger),
)
