// 职责: 使用当前 Claude Code 渠道的 OAuth 凭据读取完整的上游候选模型目录。
// 关联文件:
//
//	internal/server/biz/model_fetcher.go
//	internal/server/biz/channel_internal.go
//	internal/server/biz/channel_model_sync.go
//	llm/transformer/anthropic/claudecode/token_provider.go
//
// 注意事项:
//
//	草稿凭据只用于本次预览。预设或不完整的目录不能用于清理已有模型。

package biz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

type ModelCatalogFallbackReason string

const (
	catalogNoCredentials ModelCatalogFallbackReason = "NO_CREDENTIALS"
	catalogAuthorization ModelCatalogFallbackReason = "AUTHORIZATION"
	catalogUnavailable   ModelCatalogFallbackReason = "UNAVAILABLE"
)

func defaultClaudeCodeModels() []ModelIdentify {
	return lo.Map(claudecode.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })
}

func claudeCodePreset(reason ModelCatalogFallbackReason) *FetchModelsResult {
	return &FetchModelsResult{
		Models:         defaultClaudeCodeModels(),
		Fallback:       true,
		FallbackReason: &reason,
	}
}

func isClaudeCodeCatalogCredential(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "sk-ant-oat")
}

const claudeCodeAPI = "https://api.anthropic.com"

func (f *ModelFetcher) claudeCodeCatalog(ctx context.Context, input FetchModelsInput, raw string, saved *ent.Channel, mode modelFetchMode) *FetchModelsResult {
	// Only the interactive picker consumes this directory. Sync treats a
	// fallback as non-authoritative and leaves existing channel models alone.
	switch strings.TrimRight(input.BaseURL, "/") {
	case "", claudeCodeAPI, claudeCodeAPI + "/v1":
	default:
		return claudeCodePreset(catalogUnavailable)
	}
	if mode != previewModels {
		return claudeCodePreset(catalogUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := f.httpClient
	if input.Proxy != nil {
		client = client.WithProxy(input.Proxy)
	} else if saved != nil && saved.Settings != nil && saved.Settings.Proxy != nil {
		client = client.WithProxy(saved.Settings.Proxy)
	}
	client = client.WithRejectRedirects()
	var creds *oauth.OAuthCredentials
	var err error

	switch {
	case raw == "" && saved != nil:
		creds, err = saved.Credentials.ResolveOAuthCredentials()
	case strings.HasPrefix(strings.TrimSpace(raw), "{"):
		// Refreshing an unsaved account could rotate a token the form still needs to save.
		creds, err = oauth.ParseCredentialsJSON(raw)
	default:
		creds = &oauth.OAuthCredentials{AccessToken: strings.TrimSpace(raw)}
	}
	if err != nil || strings.TrimSpace(creds.AccessToken) == "" {
		return claudeCodePreset(catalogAuthorization)
	}
	if raw == "" && saved != nil {
		tokens := claudecode.NewTokenProvider(oauth.TokenProviderParams{
			Credentials: creds,
			HTTPClient:  client,
			OnRefreshed: f.channelService.onTokenRefreshed(saved),
		})
		creds, err = tokens.EnsureFresh(ctx, 3*time.Minute)
		if err != nil {
			if upstream, ok := xerrors.As[*httpclient.Error](err); ok && upstream.StatusCode == http.StatusBadRequest {
				return claudeCodePreset(catalogAuthorization)
			}
			return claudeCodePreset(claudeCatalogErrorReason(err))
		}
	}
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Anthropic-Version", claudecode.ClaudeCodeVersionHeader)
	headers.Set("Anthropic-Beta", "oauth-2025-04-20")
	if saved != nil {
		ApplyModelFetchHeaderOverrides(headers, channelHeaderOverrideOperations(saved))
	}
	// The catalog describes the selected credential, including on an unsaved account change.
	headers.Del("X-Api-Key")
	headers.Set("Authorization", "Bearer "+creds.AccessToken)
	models, err := readClaudeCodeCatalog(ctx, client, headers)
	if err != nil {
		return claudeCodePreset(claudeCatalogErrorReason(err))
	}
	return &FetchModelsResult{Models: models}
}

func claudeCatalogErrorReason(err error) ModelCatalogFallbackReason {
	if upstream, ok := xerrors.As[*httpclient.Error](err); ok &&
		(upstream.StatusCode == http.StatusUnauthorized || upstream.StatusCode == http.StatusForbidden) {
		return catalogAuthorization
	}
	return catalogUnavailable
}

func readClaudeCodeCatalog(ctx context.Context, client *httpclient.HttpClient, headers http.Header) ([]ModelIdentify, error) {
	query := url.Values{"limit": {"1000"}}
	var models []ModelIdentify
	seenCursors := make(map[string]bool)
	const maxPages = 10
	for range maxPages {
		resp, err := client.Do(ctx, &httpclient.Request{
			Method:  http.MethodGet,
			URL:     claudeCodeAPI + "/v1/models",
			Headers: headers,
			Query:   query,
		})
		if err != nil {
			return nil, err
		}
		var page struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore *bool  `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if resp.StatusCode != http.StatusOK || json.Unmarshal(resp.Body, &page) != nil || len(page.Data) == 0 || page.HasMore == nil {
			return nil, errors.New("invalid Claude Code model catalog page")
		}
		for _, row := range page.Data {
			id := strings.TrimSpace(row.ID)
			if id == "" {
				return nil, errors.New("missing Claude Code model ID")
			}
			models = append(models, ModelIdentify{ID: id})
		}
		if !*page.HasMore {
			return lo.UniqBy(models, func(m ModelIdentify) string { return m.ID }), nil
		}
		if page.LastID != page.Data[len(page.Data)-1].ID || seenCursors[page.LastID] {
			return nil, errors.New("invalid Claude Code model catalog cursor")
		}
		seenCursors[page.LastID] = true
		query.Set("after_id", page.LastID)
	}
	return nil, errors.New("Claude Code model catalog page limit exceeded")
}
