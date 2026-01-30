package antigravity

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// NewTokenProvider creates a new OAuth token provider for Antigravity.
func NewTokenProvider(params oauth.TokenProviderParams) *oauth.TokenProvider {
	params.OAuthUrls = DefaultTokenURLs
	if params.UserAgent == "" {
		params.UserAgent = UserAgent
	}

	params.ExchangeStrategy = &AntigravityExchangeStrategy{
		UserAgent:    params.UserAgent,
		ClientSecret: ClientSecret,
	}

	return oauth.NewTokenProvider(params)
}

// DefaultTokenURLs are the Antigravity OAuth endpoints.
var DefaultTokenURLs = oauth.OAuthUrls{
	AuthorizeUrl: AuthorizeURL,
	TokenUrl:     TokenURL,
}

// AntigravityExchangeStrategy implements ExchangeStrategy for Google OAuth which requires ClientSecret.
type AntigravityExchangeStrategy struct {
	UserAgent    string
	ClientSecret string
}

// BuildExchangeRequest implements ExchangeStrategy.
func (s *AntigravityExchangeStrategy) BuildExchangeRequest(params oauth.ExchangeParams, tokenURL string) (*httpclient.Request, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", params.ClientID)
	form.Set("client_secret", s.ClientSecret)
	form.Set("code", params.Code)
	form.Set("redirect_uri", params.RedirectURI)
	form.Set("code_verifier", params.CodeVerifier)

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if s.UserAgent != "" {
		header.Set("User-Agent", s.UserAgent)
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     tokenURL,
		Headers: header,
		Body:    []byte(form.Encode()),
	}, nil
}

// BuildRefreshRequest implements ExchangeStrategy.
func (s *AntigravityExchangeStrategy) BuildRefreshRequest(creds *oauth.OAuthCredentials, tokenURL string) (*httpclient.Request, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", creds.ClientID)
	form.Set("client_secret", s.ClientSecret)
	form.Set("refresh_token", creds.RefreshToken)

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if s.UserAgent != "" {
		header.Set("User-Agent", s.UserAgent)
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     tokenURL,
		Headers: header,
		Body:    []byte(form.Encode()),
	}, nil
}
