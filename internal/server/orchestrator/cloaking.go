package orchestrator

import (
	"context"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	entchannel "github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/cloaking"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func applyStructuredCloaking(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("cloaking-structured", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if inbound == nil || inbound.state == nil || llmRequest == nil {
			return llmRequest, nil
		}

		channel := currentStructuredCloakingChannel(inbound.state)
		if channel == nil {
			return llmRequest, nil
		}

		if channel.Type != entchannel.TypeAnthropic && channel.Type != entchannel.TypeClaudecode {
			return llmRequest, nil
		}

		effectiveMode := resolveEffectiveCloakingMode(ctx, inbound.state, channel)
		if effectiveMode == nil || *effectiveMode == "never" {
			return llmRequest, nil
		}
		if *effectiveMode != "auto" && *effectiveMode != "always" {
			return llmRequest, nil
		}

		globalConfig := resolveGlobalCloakingConfig(ctx, inbound.state)
		bodyCloak := true
		if globalConfig.BodyCloak != nil {
			bodyCloak = *globalConfig.BodyCloak
		}
		if !bodyCloak {
			return llmRequest, nil
		}

		cacheUserID := true
		if globalConfig.CacheUserID != nil {
			cacheUserID = *globalConfig.CacheUserID
		}

		clientIDHex := resolveStructuredCloakingClientIDHex(ctx, inbound.state, channel)

		cacheControlAutoInject := true
		if globalConfig.CacheControlAutoInject != nil {
			cacheControlAutoInject = *globalConfig.CacheControlAutoInject
		}

		sensitiveWordsMode := ""
		if globalConfig.SensitiveWordsMode != nil {
			sensitiveWordsMode = *globalConfig.SensitiveWordsMode
		}

		processed := cloaking.ApplyStructuredProcessor(shared.WithChannelID(ctx, channel.ID), *llmRequest, cloaking.ProcessorConfig{
			ChannelType:            string(channel.Type),
			IsOfficial:             channel.Credentials.IsOAuth(),
			CacheUserID:            cacheUserID,
			ClientIDHex:            clientIDHex,
			CacheControlAutoInject: cacheControlAutoInject,
			SensitiveWordsMode:     sensitiveWordsMode,
			SensitiveWords:         globalConfig.SensitiveWords,
		})

		return &processed, nil
	})
}

func currentStructuredCloakingChannel(state *PersistenceState) *biz.Channel {
	if state == nil {
		return nil
	}
	if state.CurrentCandidate != nil {
		return state.CurrentCandidate.Channel
	}
	if len(state.ChannelModelsCandidates) == 0 {
		return nil
	}
	return state.ChannelModelsCandidates[state.CurrentCandidateIndex].Channel
}

func resolveEffectiveCloakingMode(ctx context.Context, state *PersistenceState, channel *biz.Channel) *string {
	if channel != nil && channel.Settings != nil {
		if mode := channel.Settings.CloakingMode; mode != nil && *mode != "follow_global" {
			return mode
		}
	}
	return resolveGlobalCloakingConfig(ctx, state).Mode
}

func resolveGlobalCloakingConfig(ctx context.Context, state *PersistenceState) *biz.GlobalCloakingConfig {
	if state == nil || state.ChannelService == nil || state.ChannelService.SystemService == nil {
		return &biz.GlobalCloakingConfig{}
	}
	config, err := state.ChannelService.SystemService.GlobalCloakingConfig(ctx)
	if err != nil {
		return &biz.GlobalCloakingConfig{}
	}
	return config
}

func resolveStructuredCloakingClientIDHex(ctx context.Context, state *PersistenceState, channel *biz.Channel) string {
	if state == nil || channel == nil {
		return ""
	}

	entClient := ent.FromContext(ctx)
	if entClient == nil {
		return ""
	}

	principalKind := "api_key"
	principalKey := ""
	if channel.Credentials.IsOAuth() {
		principalKind = "oauth"
	} else if state.APIKey != nil {
		principalKey = strings.TrimSpace(state.APIKey.Key)
	}

	if principalKind == "api_key" && principalKey == "" {
		return ""
	}

	principalHash := biz.ComputePrincipalHash(principalKind, principalKey)
	service := biz.NewChannelClientIDService(entClient)
	clientIDHex, err := service.GetOrCreateClientID(ctx, channel.ID, principalKind, principalHash)
	if err != nil {
		return ""
	}

	return clientIDHex
}
