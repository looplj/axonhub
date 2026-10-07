package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
)

// fetchAntigravityModels queries the account catalog rather than a static model list.
// Like Codex discovery, it reuses saved credentials, proxy settings and token persistence.
func (f *ModelFetcher) fetchAntigravityModels(ctx context.Context, input FetchModelsInput) ([]ModelIdentify, error) {
	httpClient := f.httpClient
	var ch *ent.Channel
	var credentials *oauth.OAuthCredentials
	var onRefreshed func(context.Context, *oauth.OAuthCredentials) error
	raw := ""
	if input.APIKey != nil {
		raw = strings.TrimSpace(*input.APIKey)
	}
	if input.ChannelID != nil {
		var err error
		ch, err = f.channelService.entFromContext(ctx).Channel.Get(ctx, *input.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("failed to get Antigravity channel: %w", err)
		}
		if ch.Settings != nil && ch.Settings.Proxy != nil {
			httpClient = httpClient.WithProxy(ch.Settings.Proxy)
		}
		if raw == "" {
			if !fetchModelsInputMatchesChannel(input, ch) {
				return nil, fmt.Errorf("credentials are required when channel type or base URL is changed")
			}
			raw = ch.Credentials.APIKey
		}
		// Edited credentials must not refresh or overwrite the saved account.
		if raw == ch.Credentials.APIKey && fetchModelsInputMatchesChannel(input, ch) {
			refreshToken, _, _ := strings.Cut(strings.TrimSpace(raw), "|")
			saved := ch.Credentials.OAuth
			// Create/update/restore may retain OAuth data from a different account.
			// Only reuse and persist a verified pair; otherwise resolve the APIKey
			// independently without writing the saved account's credentials.
			// Without a legacy APIKey, saved OAuth is the channel's sole identity.
			if saved == nil || (saved.AccessToken == "" && saved.RefreshToken == "") ||
				(refreshToken != "" && saved.RefreshToken == refreshToken) || ch.Credentials.APIKey == "" {
				credentials = saved
				onRefreshed = f.channelService.onTokenRefreshed(ch)
			}
		}
	}

	projectID := ""
	if strings.HasPrefix(strings.TrimSpace(raw), "{") {
		parsed, err := oauth.ParseCredentialsJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Antigravity OAuth credentials: %w", err)
		}
		credentials = parsed
		var project struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal([]byte(raw), &project); err != nil {
			return nil, fmt.Errorf("failed to parse Antigravity project: %w", err)
		}
		projectID = project.ProjectID
		// Legacy Antigravity persistence requires refreshToken|projectID.
		onRefreshed = nil
	} else {
		parts := strings.SplitN(strings.TrimSpace(raw), "|", 2)
		if len(parts) == 2 {
			projectID = parts[1]
		}
		if credentials == nil {
			credentials = &oauth.OAuthCredentials{RefreshToken: parts[0]}
		}
	}
	credentialsCopy := *credentials
	if credentialsCopy.RefreshToken == "" && !strings.HasPrefix(raw, "{") {
		credentialsCopy.RefreshToken = strings.SplitN(raw, "|", 2)[0]
	}
	if credentialsCopy.AccessToken == "" && credentialsCopy.RefreshToken == "" {
		return nil, fmt.Errorf("Antigravity OAuth credentials are required to fetch models")
	}
	if projectID == "" {
		onRefreshed = nil
	}
	if credentialsCopy.ClientID == "" {
		credentialsCopy.ClientID = antigravity.ClientID
	}
	if len(credentialsCopy.Scopes) == 0 {
		credentialsCopy.Scopes = antigravity.Scopes
	}
	tokens := antigravity.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: &credentialsCopy,
		HTTPClient:  httpClient,
		OnRefreshed: onRefreshed,
	})
	refreshed, err := tokens.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Antigravity OAuth token: %w", err)
	}
	body := map[string]string{}
	if projectID != "" {
		body["project"] = projectID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Antigravity model request: %w", err)
	}
	headers := http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}}
	antigravity.SetClientHeaders(headers)
	headers.Set("Authorization", "Bearer "+refreshed.AccessToken)
	response, err := httpClient.Do(ctx, &httpclient.Request{
		Method:  http.MethodPost,
		URL:     antigravity.EndpointProd + "/v1internal:fetchAvailableModels",
		Headers: headers,
		Body:    encoded,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Antigravity model catalog: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Antigravity model catalog: status %d", response.StatusCode)
	}
	var catalog struct {
		Models map[string]struct {
			IsInternal bool `json:"isInternal"`
		} `json:"models"`
	}
	if err := json.Unmarshal(response.Body, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse Antigravity model catalog: %w", err)
	}
	models := make([]ModelIdentify, 0, len(catalog.Models))
	for id, model := range catalog.Models {
		if strings.TrimSpace(id) != "" && !model.IsInternal {
			models = append(models, ModelIdentify{ID: id})
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	if len(models) == 0 {
		return nil, fmt.Errorf("Antigravity model catalog contains no available public models")
	}
	return models, nil
}
