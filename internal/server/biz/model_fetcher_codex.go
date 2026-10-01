package biz

import (
	"context"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func (f *ModelFetcher) fetchCodexModels(ctx context.Context, ch *ent.Channel) *FetchModelsResult {
	fallback := &FetchModelsResult{Models: f.getDefaultModelsByType(ctx, channel.TypeCodex), Fallback: true}
	client := f.httpClient
	var operations []objects.OverrideOperation
	if ch.Settings != nil {
		if ch.Settings.Proxy != nil {
			client = client.WithProxy(ch.Settings.Proxy)
		}
		operations = ch.Settings.HeaderOverrideOperations
		if operations == nil {
			operations = objects.HeaderEntriesToOverrideOperations(ch.Settings.OverrideHeaders)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	creds, err := ch.Credentials.ResolveOAuthCredentials()
	if err != nil {
		return fallback
	}
	params := codex.TokenProviderParams{Credentials: creds, HTTPClient: client}
	if ch.ID != 0 {
		params.OnRefreshed = f.channelService.onTokenRefreshed(ch)
	}
	outbound, err := codex.NewOutboundTransformer(codex.Params{TokenProvider: codex.NewTokenProvider(params), BaseURL: ch.BaseURL})
	if err != nil {
		return fallback
	}
	req, err := outbound.ModelsRequest(ctx)
	if err != nil {
		return fallback
	}
	ApplyModelFetchHeaderOverrides(req.Headers, operations)
	resp, err := client.WithRejectHTTPSDowngrade().Do(ctx, req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fallback
	}
	models, err := codex.ParseModelCatalog(resp.Body)
	if err != nil {
		return fallback
	}
	return &FetchModelsResult{Models: lo.Map(lo.Uniq(models), func(id string, _ int) ModelIdentify {
		return ModelIdentify{ID: id}
	})}
}
