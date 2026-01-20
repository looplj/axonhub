package codex

import (
	"context"

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

func NewTokenProvider(params TokenProviderParams) *oauth.TokenProvider {
	return oauth.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: params.Credentials,
		HTTPClient:  params.HTTPClient,
		OAuthUrls:   DefaultTokenURLs,
		UserAgent:   UserAgent,
		OnRefreshed: params.OnRefreshed,
	})
}
