package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

type CodexCatalogError struct {
	Status         int
	UpstreamStatus int
	Message        string
}

func (e *CodexCatalogError) Error() string { return e.Message }

type CodexCatalog struct {
	fields map[string]json.RawMessage
	models []json.RawMessage
	slugs  []string
}

func ParseCodexCatalog(body []byte) (*CodexCatalog, error) {
	invalid := &CodexCatalogError{Status: 502, Message: "invalid Codex model catalog"}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return nil, invalid
	}
	var models []json.RawMessage
	if err := json.Unmarshal(fields["models"], &models); err != nil || models == nil {
		return nil, invalid
	}
	slugs := make([]string, len(models))
	for i, raw := range models {
		var model struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(raw, &model); err != nil || model.Slug == "" {
			return nil, invalid
		}
		slugs[i] = model.Slug
	}
	return &CodexCatalog{fields: fields, models: models, slugs: slugs}, nil
}

func (c *CodexCatalog) Count() int { return len(c.models) }

func (c *CodexCatalog) Intersect(ids []string) ([]byte, error) {
	visible := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		visible[id] = struct{}{}
	}
	models := make([]json.RawMessage, 0, len(c.models))
	for i, slug := range c.slugs {
		if _, ok := visible[slug]; ok {
			models = append(models, c.models[i])
		}
	}
	fields := make(map[string]json.RawMessage, len(c.fields))
	maps.Copy(fields, c.fields)
	raw, err := json.Marshal(models)
	if err != nil {
		return nil, fmt.Errorf("encode catalog intersection: %w", err)
	}
	fields["models"] = raw
	return json.Marshal(fields)
}

func (svc *ChannelService) FetchCodexCatalog(ctx context.Context, channelID int, version string) (*CodexCatalog, error) {
	result := svc.codexCatalogSF.DoChan(fmt.Sprintf("%d:%s", channelID, version), func() (any, error) {
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		fetchCtx = ent.NewContext(fetchCtx, svc.db)
		return svc.fetchCodexCatalog(fetchCtx, channelID, version)
	})
	select {
	case <-ctx.Done():
		return nil, &CodexCatalogError{Status: http.StatusBadGateway, Message: "Codex catalog request was cancelled"}
	case response := <-result:
		if response.Err != nil {
			return nil, response.Err
		}
		catalog, ok := response.Val.(*CodexCatalog)
		if !ok {
			return nil, &CodexCatalogError{Status: http.StatusBadGateway, Message: "Codex catalog response is invalid"}
		}
		return catalog, nil
	}
}

func (svc *ChannelService) fetchCodexCatalog(ctx context.Context, channelID int, version string) (*CodexCatalog, error) {
	entity, err := svc.db.Channel.Get(ctx, channelID)
	if err != nil || entity.Type != channel.TypeCodex {
		return nil, &CodexCatalogError{Status: 503, Message: "Codex catalog source is unavailable"}
	}
	source := svc.GetEnabledChannel(channelID)
	var outbound *codex.OutboundTransformer
	var httpClient *httpclient.HttpClient
	if source != nil {
		var ok bool
		outbound, ok = source.Outbound.(*codex.OutboundTransformer)
		if !ok {
			return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog provider is unavailable"}
		}
		httpClient = source.HTTPClient
	} else {
		outbound, httpClient, err = svc.codexCatalogOutbound(ctx, entity)
		if err != nil {
			return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog credentials are unavailable"}
		}
	}
	baseURL := entity.BaseURL
	switch {
	case strings.HasPrefix(baseURL, "wss://"):
		baseURL = "https://" + strings.TrimPrefix(baseURL, "wss://")
	case strings.HasPrefix(baseURL, "ws://"):
		baseURL = "http://" + strings.TrimPrefix(baseURL, "ws://")
	}
	req, err := codex.ModelsRequest(ctx, outbound.TokenProvider(), baseURL)
	if err != nil {
		return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog authentication failed"}
	}
	parsed, err := url.Parse(req.URL)
	if err != nil {
		return nil, &CodexCatalogError{Status: 502, Message: "invalid Codex catalog URL"}
	}
	query := parsed.Query()
	if version != "" {
		query.Set("client_version", version)
	}
	parsed.RawQuery = query.Encode()
	req.URL = parsed.String()
	native := *httpClient.GetNativeClient()
	previous := native.CheckRedirect
	native.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 0 && (next.URL.Scheme != via[0].URL.Scheme || next.URL.Host != via[0].URL.Host) {
			return http.ErrUseLastResponse
		}
		if previous != nil {
			return previous(next, via)
		}
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}
	resp, err := httpclient.NewHttpClientWithClient(&native).Do(ctx, req)
	if err != nil {
		if upstreamErr, ok := errors.AsType[*httpclient.Error](err); ok {
			return nil, &CodexCatalogError{Status: 502, UpstreamStatus: upstreamErr.StatusCode, Message: fmt.Sprintf("Codex catalog upstream HTTP %d", upstreamErr.StatusCode)}
		}
		return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog transport failed"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &CodexCatalogError{Status: 502, UpstreamStatus: resp.StatusCode, Message: fmt.Sprintf("Codex catalog upstream HTTP %d", resp.StatusCode)}
	}
	return ParseCodexCatalog(resp.Body)
}

type codexCatalogOutboundEntry struct {
	signature string
	outbound  *codex.OutboundTransformer
}

func (svc *ChannelService) codexCatalogOutbound(ctx context.Context, entity *ent.Channel) (*codex.OutboundTransformer, *httpclient.HttpClient, error) {
	httpClient := svc.getHttpClient(entity.Settings)
	if !entity.Credentials.IsOAuth() {
		source, err := svc.buildChannelWithOutbounds(entity)
		if err != nil {
			return nil, nil, err
		}
		outbound, ok := source.Outbound.(*codex.OutboundTransformer)
		if !ok {
			return nil, nil, errors.New("codex catalog provider is unavailable")
		}
		return outbound, httpClient, nil
	}

	credentials, err := entity.Credentials.ResolveOAuthCredentials()
	if err != nil {
		return nil, nil, err
	}
	signature := credentials.AccessToken + "\x00" + credentials.RefreshToken + "\x00" + credentials.ExpiresAt.String()
	if cached, ok := svc.codexCatalogOutbounds.Load(entity.ID); ok {
		entry, ok := cached.(codexCatalogOutboundEntry)
		if ok && entry.signature == signature {
			return entry.outbound, httpClient, nil
		}
	}

	result := svc.codexCatalogProviderSF.DoChan(fmt.Sprintf("%d:%s", entity.ID, signature), func() (any, error) {
		if cached, ok := svc.codexCatalogOutbounds.Load(entity.ID); ok {
			entry, ok := cached.(codexCatalogOutboundEntry)
			if ok && entry.signature == signature {
				return entry.outbound, nil
			}
		}
		outbound, err := svc.buildCodexOutboundWithRefresh(
			entity,
			nil,
			entity.BaseURL,
			primaryEndpointTransport(entity, llm.APIFormatOpenAIResponse.String()),
			"",
			httpClient,
			svc.onTokenRefreshedDetached(entity),
		)
		if err != nil {
			return nil, err
		}
		codexOutbound, ok := outbound.(*codex.OutboundTransformer)
		if !ok {
			return nil, errors.New("codex catalog provider is unavailable")
		}
		svc.codexCatalogOutbounds.Store(entity.ID, codexCatalogOutboundEntry{signature: signature, outbound: codexOutbound})
		return codexOutbound, nil
	})
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case response := <-result:
		if response.Err != nil {
			return nil, nil, response.Err
		}
		outbound, ok := response.Val.(*codex.OutboundTransformer)
		if !ok {
			return nil, nil, errors.New("codex catalog provider is unavailable")
		}
		return outbound, httpClient, nil
	}
}

func (svc *ChannelService) onTokenRefreshedDetached(ch *ent.Channel) func(context.Context, *oauth.OAuthCredentials) error {
	return func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
		ctx = authz.WithSystemBypass(context.WithoutCancel(ctx), "codex-catalog-refresh")
		return svc.refreshOAuthTokenWithClient(ctx, svc.db, ch, refreshed)
	}
}
