package copilot

import (
	"context"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// CopilotTokenProvider manages OAuth2 credentials and exchanges them for Copilot tokens.
// It wraps oauth.DeviceFlowProvider internally to handle the device flow lifecycle
// and token exchange for Copilot-specific tokens.
type CopilotTokenProvider struct {
	deviceFlowProvider *oauth.DeviceFlowProvider
}

// TokenProviderParams contains the parameters for creating a new CopilotTokenProvider.
type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	HTTPClient  *httpclient.HttpClient
	OnRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

// NewTokenProvider creates a new CopilotTokenProvider instance.
// It wraps a DeviceFlowProvider to handle the device flow lifecycle and token exchange.
func NewTokenProvider(params TokenProviderParams) (*CopilotTokenProvider, error) {
	exchanger := NewTokenExchanger(TokenExchangerParams{
		HTTPClient: params.HTTPClient,
	})

	config := oauth.DeviceFlowConfig{ //nolint:gosec
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
		TokenExchanger: exchanger,
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
