package codex

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// DefaultTokenURLs are the production OpenAI OAuth endpoints.
var DefaultTokenURLs = oauth.OAuthUrls{
	AuthorizeUrl: AuthorizeURL,
	TokenUrl:     TokenURL,
}

type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	HTTPClient  *httpclient.HttpClient
	OnRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

type AuthJSON struct {
	LastRefresh string `json:"last_refresh,omitempty"`
	Tokens      struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		IDToken      string `json:"id_token,omitempty"`
	} `json:"tokens"`
}

func DecodeAuthJSON(raw string) (*oauth.OAuthCredentials, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("empty auth json")
	}

	var authJSON AuthJSON
	if err := json.Unmarshal([]byte(trimmed), &authJSON); err != nil {
		return nil, err
	}

	if strings.TrimSpace(authJSON.Tokens.AccessToken) == "" {
		return nil, errors.New("access_token is empty")
	}

	creds := &oauth.OAuthCredentials{
		ClientID:     ClientID,
		AccessToken:  authJSON.Tokens.AccessToken,
		RefreshToken: authJSON.Tokens.RefreshToken,
		IDToken:      authJSON.Tokens.IDToken,
		TokenType:    "bearer",
		Scopes:       strings.Fields(Scopes),
	}

	if authJSON.LastRefresh != "" {
		lastRefresh, err := time.Parse(time.RFC3339Nano, authJSON.LastRefresh)
		if err == nil {
			creds.ExpiresAt = lastRefresh.Add(1 * time.Hour)
		}
	}

	if creds.RefreshToken != "" && creds.ExpiresAt.IsZero() {
		creds.ExpiresAt = time.Now().Add(1 * time.Hour)
	}

	return creds, nil
}

func NewTokenProvider(params TokenProviderParams) *oauth.TokenProvider {
	return oauth.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: params.Credentials,
		HTTPClient:  params.HTTPClient,
		OAuthUrls:   DefaultTokenURLs,
		OnRefreshed: params.OnRefreshed,
	})
}
