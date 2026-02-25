package copilot

import (
	"context"
	"errors"

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

// copilotTokenExchanger adapts the local TokenExchanger interface to oauth.TokenExchanger.
// This allows the DeviceFlowProvider to use the existing CopilotTokenExchanger implementation.
type copilotTokenExchanger struct {
	exchanger  TokenExchanger
	httpClient *httpclient.HttpClient
}

// Exchange implements oauth.TokenExchanger.Exchange.
func (c *copilotTokenExchanger) Exchange(ctx context.Context, accessToken string) (string, int64, error) {
	return c.exchanger.GetToken(ctx, accessToken)
}

// ExchangeWithClient implements oauth.TokenExchanger.ExchangeWithClient.
func (c *copilotTokenExchanger) ExchangeWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (string, int64, error) {
	// Use the provided httpClient if available, otherwise fall back to the one from the adapter
	client := httpClient
	if client == nil {
		client = c.httpClient
	}
	return c.exchanger.GetTokenWithClient(ctx, client, accessToken)
}

// CopilotTokenProvider manages OAuth2 credentials and exchanges them for Copilot tokens.
// It wraps oauth.DeviceFlowProvider internally to handle the device flow lifecycle
// and token exchange for Copilot-specific tokens.
type CopilotTokenProvider struct {
	deviceFlowProvider *oauth.DeviceFlowProvider
}

// TokenProviderParams contains the parameters for creating a new CopilotTokenProvider.
type TokenProviderParams struct {
	Credentials    *oauth.OAuthCredentials
	HTTPClient     *httpclient.HttpClient
	TokenExchanger TokenExchanger
	OnRefreshed    func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

// NewTokenProvider creates a new CopilotTokenProvider instance.
// It wraps a DeviceFlowProvider to handle the device flow lifecycle and token exchange.
// Returns an error if TokenExchanger is nil.
func NewTokenProvider(params TokenProviderParams) (*CopilotTokenProvider, error) {
	if params.TokenExchanger == nil {
		return nil, errors.New("TokenExchanger is required")
	}

	adapter := &copilotTokenExchanger{
		exchanger:  params.TokenExchanger,
		httpClient: params.HTTPClient,
	}

	config := oauth.DeviceFlowConfig{
		DeviceAuthURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
		ClientID:      "Iv1.b507a08c87ecfe98",
		Scopes:        []string{"read:user"},
		UserAgent:     "",
	}

	deviceFlowProvider := oauth.NewDeviceFlowProvider(oauth.DeviceFlowProviderParams{
		Config:         config,
		HTTPClient:     params.HTTPClient,
		Credentials:    params.Credentials,
		TokenExchanger: adapter,
		OnRefreshed:    params.OnRefreshed,
	})

	return &CopilotTokenProvider{
		deviceFlowProvider: deviceFlowProvider,
	}, nil
}

// GetToken returns a valid Copilot token.
// If the cached copilot token is expired or missing, it exchanges the access token for a new one.
// This method implements the token provider interface used by the Copilot outbound transformer.
func (p *CopilotTokenProvider) GetToken(ctx context.Context) (string, error) {
	if p.deviceFlowProvider == nil {
		return "", errors.New("device flow provider is nil")
	}
	return p.deviceFlowProvider.GetToken(ctx)
}

// UpdateCredentials updates the stored OAuth credentials.
// This is called when new credentials are obtained (e.g., after device flow completes).
// Delegates to the underlying DeviceFlowProvider.
func (p *CopilotTokenProvider) UpdateCredentials(creds *oauth.OAuthCredentials) {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.UpdateCredentials(creds)
	}
}

// GetCredentials returns a copy of the current OAuth credentials.
// Returns nil if no credentials are stored.
// Delegates to the underlying DeviceFlowProvider.
func (p *CopilotTokenProvider) GetCredentials() *oauth.OAuthCredentials {
	if p.deviceFlowProvider == nil {
		return nil
	}
	return p.deviceFlowProvider.GetCredentials()
}

// StartAutoRefresh starts automatic background token refresh.
// The token will be refreshed before it expires based on the provided options.
func (p *CopilotTokenProvider) StartAutoRefresh(ctx context.Context, opts oauth.AutoRefreshOptions) {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.StartAutoRefresh(ctx, opts)
	}
}

// StopAutoRefresh stops automatic token refresh.
func (p *CopilotTokenProvider) StopAutoRefresh() {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.StopAutoRefresh()
	}
}
