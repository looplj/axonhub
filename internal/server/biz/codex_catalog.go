package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
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
	for key, value := range c.fields {
		fields[key] = value
	}
	raw, err := json.Marshal(models)
	if err != nil {
		return nil, fmt.Errorf("encode catalog intersection: %w", err)
	}
	fields["models"] = raw
	return json.Marshal(fields)
}

func (svc *ChannelService) FetchCodexCatalog(ctx context.Context, channelID int, version string) (*CodexCatalog, error) {
	mutex, _ := svc.codexCatalogLocks.LoadOrStore(channelID, new(sync.Mutex))
	lock := mutex.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	entity, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil || entity.Type != channel.TypeCodex {
		return nil, &CodexCatalogError{Status: 503, Message: "Codex catalog source is unavailable"}
	}
	source := svc.GetEnabledChannel(channelID)
	if source == nil {
		source, err = svc.GetChannel(ctx, channelID)
		if err != nil {
			return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog credentials are unavailable"}
		}
	}
	outbound, ok := source.Outbound.(*codex.OutboundTransformer)
	if !ok {
		return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog provider is unavailable"}
	}
	req, err := codex.ModelsRequest(ctx, outbound.TokenProvider(), entity.BaseURL)
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
	native := *source.HTTPClient.GetNativeClient()
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
		var upstream *httpclient.Error
		if errors.As(err, &upstream) {
			return nil, &CodexCatalogError{Status: 502, UpstreamStatus: upstream.StatusCode, Message: fmt.Sprintf("Codex catalog upstream HTTP %d", upstream.StatusCode)}
		}
		return nil, &CodexCatalogError{Status: 502, Message: "Codex catalog transport failed"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &CodexCatalogError{Status: 502, UpstreamStatus: resp.StatusCode, Message: fmt.Sprintf("Codex catalog upstream HTTP %d", resp.StatusCode)}
	}
	return ParseCodexCatalog(resp.Body)
}
