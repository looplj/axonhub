package copilot

import (
	"context"
	"errors"
	"sync"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// TokenExchanger defines the interface for exchanging OAuth access tokens for Copilot tokens.
// This interface is typically implemented by biz.CopilotTokenExchanger.
type TokenExchanger interface {
	// GetToken returns a Copilot token for the given access token.
	// It handles caching internally and returns the token with its expiration timestamp.
	GetToken(ctx context.Context, accessToken string) (string, int64, error)

	// GetTokenWithClient returns a Copilot token for the given access token using the provided HTTP client.
	// This allows the caller to specify a custom HTTP client (e.g., with proxy settings).
	// If the implementation doesn't support custom clients, it may fall back to its default client.
	GetTokenWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (string, int64, error)
}

// CopilotTokenProvider manages OAuth2 credentials and exchanges them for Copilot tokens.
// Unlike standard OAuth providers that use refresh tokens, Copilot uses a two-step flow:
// 1. Device flow -> access_token (stored in OAuthCredentials)
// 2. Token exchange -> copilot_token (via TokenExchanger)
type CopilotTokenProvider struct {
	httpClient     *httpclient.HttpClient
	tokenExchanger TokenExchanger
	credentials    *oauth.OAuthCredentials
	mu             sync.RWMutex
}

// TokenProviderParams contains the parameters for creating a new CopilotTokenProvider.
type TokenProviderParams struct {
	Credentials    *oauth.OAuthCredentials
	HTTPClient     *httpclient.HttpClient
	TokenExchanger TokenExchanger
}

// NewTokenProvider creates a new CopilotTokenProvider instance.
// It wraps a TokenExchanger to handle the token exchange lifecycle.
// Returns an error if TokenExchanger is nil.
func NewTokenProvider(params TokenProviderParams) (*CopilotTokenProvider, error) {
	if params.TokenExchanger == nil {
		return nil, errors.New("TokenExchanger is required")
	}
	return &CopilotTokenProvider{
		httpClient:     params.HTTPClient,
		tokenExchanger: params.TokenExchanger,
		credentials:    params.Credentials,
	}, nil
}

// GetToken returns a valid Copilot token.
// If the cached copilot token is expired or missing, it exchanges the access token for a new one.
// This method implements the token provider interface used by the Copilot outbound transformer.
func (p *CopilotTokenProvider) GetToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	creds := p.credentials
	p.mu.RUnlock()

	if creds == nil {
		return "", errors.New("credentials is nil")
	}

	if creds.AccessToken == "" {
		return "", errors.New("access token is empty")
	}

	// The TokenExchanger handles caching internally
	// It returns cached token if valid, or exchanges for a new one if expired
	// Use GetTokenWithClient to honor the channel's httpClient (with proxy settings)
	token, _, err := p.tokenExchanger.GetTokenWithClient(ctx, p.httpClient, creds.AccessToken)
	if err != nil {
		return "", err
	}

	return token, nil
}
// UpdateCredentials updates the stored OAuth credentials.
// This is called when new credentials are obtained (e.g., after device flow completes).
// Stores a shallow copy to prevent concurrent mutation.
func (p *CopilotTokenProvider) UpdateCredentials(creds *oauth.OAuthCredentials) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if creds != nil {
		c := *creds
		p.credentials = &c
	} else {
		p.credentials = nil
	}
}

// GetCredentials returns a shallow copy of the current OAuth credentials.
// Returns nil if no credentials are stored.
func (p *CopilotTokenProvider) GetCredentials() *oauth.OAuthCredentials {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.credentials != nil {
		credCopy := *p.credentials
		return &credCopy
	}
	return nil
}
