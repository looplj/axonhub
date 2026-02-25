package api

import (
	"go.uber.org/fx"
)

var Module = fx.Module("api",
	fx.Provide(NewOpenAIHandlers),
	fx.Provide(NewAnthropicHandlers),
	fx.Provide(NewGeminiHandlers),
	fx.Provide(NewAiSDKHandlers),
	fx.Provide(NewPlaygroundHandlers),
	fx.Provide(NewSystemHandlers),
	fx.Provide(NewAuthHandlers),
	fx.Provide(NewAPIKeyHandlers),
	fx.Provide(NewJinaHandlers),
	fx.Provide(NewDoubaoHandlers),
	fx.Provide(NewCodexHandlers),
	fx.Provide(NewClaudeCodeHandlers),
	fx.Provide(NewAntigravityHandlers),
	fx.Provide(NewRequestContentHandlers),
	fx.Invoke(initLogger),
)
