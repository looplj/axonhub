package cloaking

import (
	"context"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

type ProcessorConfig struct {
	ChannelType            string
	IsOfficial             bool
	CacheUserID            bool
	ClientIDHex            string
	CacheControlAutoInject bool
	SensitiveWordsMode     string
	SensitiveWords         *[]string
}

func ApplyStructuredProcessor(ctx context.Context, req llm.Request, cfg ProcessorConfig) llm.Request {
	if cfg.ChannelType == "claudecode" {
		if cfg.IsOfficial {
			req = *claudecode.EnsureBillingSystemMessageCCH(&req)
		}
		req = *claudecode.InjectClaudeCodeSystemMessageStructured(&req)
		if cfg.CacheUserID {
			req = claudecode.InjectFakeUserIDStructuredWithClientID(ctx, req, cfg.ClientIDHex)
		}
	}

	if req.TransformerMetadata == nil {
		req.TransformerMetadata = make(map[string]any)
	}
	req.TransformerMetadata[llm.TransformerMetadataKeyCloakingCacheControlAutoInject] = cfg.CacheControlAutoInject

	mode := cfg.SensitiveWordsMode
	if mode == "" {
		mode = "extend"
	}

	return ApplySensitiveWords(ctx, req, SensitiveWordsConfig{
		Mode:           mode,
		SensitiveWords: cfg.SensitiveWords,
	})
}
