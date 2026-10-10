package server

import (
	"testing"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/stretchr/testify/require"
)

func setupCodexResponseHandlers(t *testing.T, h *codexRouteHarness, models *biz.ModelService) (*api.OpenAIHandlers, *biz.RequestService) {
	t.Helper()
	cache := xcache.Config{Mode: xcache.ModeMemory}
	h.db.DataStorage.Create().SetName("test").SetDescription("test").SetPrimary(true).SetType(datastorage.TypeDatabase).SetSettings(&objects.DataStorageSettings{}).SaveX(h.ctx)
	storage := &biz.DataStorageService{AbstractService: &biz.AbstractService{}, SystemService: h.system, Cache: xcache.NewFromConfig[ent.DataStorage](cache)}
	usage := biz.NewUsageLogService(h.db, h.system, h.channels)
	requests := biz.NewRequestService(h.db, cache, h.system, usage, storage, biz.NewLiveStreamRegistry())
	prompts := biz.NewPromptService(biz.PromptServiceParams{Ent: h.db})
	protection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{Ent: h.db, CacheConfig: cache})
	t.Cleanup(protection.Stop)
	selector := orchestrator.NewDefaultSelector(h.channels, models, h.system)
	orch := orchestrator.NewChatCompletionOrchestrator(h.channels, selector, requests, httpclient.NewHttpClient(), responses.NewInboundTransformer(), h.system, usage, prompts, nil, protection, biz.NewLiveStreamRegistry(), orchestrator.NewChannelLimiterManager(), nil)
	require.NoError(t, h.system.SetModelSettings(h.ctx, biz.SystemModelSettings{QueryAllChannelModels: true, FallbackToChannelsOnModelNotFound: true}))
	return &api.OpenAIHandlers{ResponseCompletionHandlers: api.NewChatCompletionHandlers(orch), ModelService: models, SystemService: h.system, EntClient: h.db}, requests
}
