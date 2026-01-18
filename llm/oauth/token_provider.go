package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
)

type OAuthUrls struct {
	AuthorizeUrl string
	TokenUrl     string
}

// TokenProvider manages OAuth2 credentials for a transformer instance.
// Each transformer has its own provider, so we can keep the credentials in memory.
type TokenProvider struct {
	httpClient  *httpclient.HttpClient
	oauthUrls   OAuthUrls
	sf          singleflight.Group
	mu          sync.RWMutex
	creds       *OAuthCredentials
	userAgent   string
	onRefreshed func(ctx context.Context, refreshed *OAuthCredentials) error
}

type TokenProviderParams struct {
	Credentials *OAuthCredentials
	// HTTPClient should be pre-configured with proxy settings if needed
	HTTPClient  *httpclient.HttpClient
	OAuthUrls   OAuthUrls
	UserAgent   string
	OnRefreshed func(ctx context.Context, refreshed *OAuthCredentials) error
}
type ExchangeParams struct {
	Code         string
	CodeVerifier string
	ClientID     string
	RedirectURI  string
}

func NewTokenProvider(params TokenProviderParams) *TokenProvider {
	return &TokenProvider{
		httpClient:  params.HTTPClient,
		oauthUrls:   params.OAuthUrls,
		userAgent:   params.UserAgent,
		creds:       params.Credentials,
		onRefreshed: params.OnRefreshed,
	}
}

// Exchange performs OAuth2 authorization_code exchange and returns credentials.
func (p *TokenProvider) Exchange(ctx context.Context, params ExchangeParams) (*OAuthCredentials, error) {
	if p.httpClient == nil {
		return nil, errors.New("http client is nil")
	}

	if p.oauthUrls.TokenUrl == "" {
		return nil, errors.New("token URL is empty")
	}

	if params.Code == "" {
		return nil, errors.New("code is empty")
	}

	if params.CodeVerifier == "" {
		return nil, errors.New("code_verifier is empty")
	}

	if params.ClientID == "" {
		return nil, errors.New("client_id is empty")
	}

	if params.RedirectURI == "" {
		return nil, errors.New("redirect_uri is empty")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", params.ClientID)
	form.Set("code", params.Code)
	form.Set("redirect_uri", params.RedirectURI)
	form.Set("code_verifier", params.CodeVerifier)

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if p.userAgent != "" {
		header.Set("User-Agent", p.userAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.oauthUrls.TokenUrl,
		Headers: header,
		Body:    []byte(form.Encode()),
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode exchange response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		var tokenErr TokenError
		if err := json.Unmarshal(resp.Body, &tokenErr); err == nil && tokenErr.Error != "" {
			return nil, fmt.Errorf("token exchange failed: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
		}

		return nil, errors.New("token exchange response missing required fields")
	}

	now := time.Now()
	creds := &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     params.ClientID,
		TokenType:    tokenResp.TokenType,
	}

	if tokenResp.Scope != "" {
		creds.Scopes = strings.Fields(tokenResp.Scope)
	}

	if tokenResp.ExpiresIn > 0 {
		creds.ExpiresAt = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// Get returns valid OAuth2 credentials.
// It refreshes them if expired.
func (p *TokenProvider) Get(ctx context.Context) (*OAuthCredentials, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return nil, fmt.Errorf("credentials is nil")
	}

	now := time.Now()
	if !creds.IsExpired(now) {
		return creds, nil
	}

	// Refresh with singleflight to avoid stampede inside the same transformer.
	v, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		onRefreshed := p.onRefreshed
		p.mu.RUnlock()

		if current == nil {
			return nil, fmt.Errorf("credentials is nil")
		}

		if !current.IsExpired(time.Now()) {
			return current, nil
		}

		fresh, err := p.refresh(ctx, current)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.creds = fresh
		p.mu.Unlock()

		if onRefreshed != nil {
			if err := onRefreshed(ctx, fresh); err != nil {
				log.Warn(ctx, "failed to persist refreshed  credentials", log.Cause(err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh, nil
}

// refresh performs the OAuth2 token refresh flow.
func (p *TokenProvider) refresh(ctx context.Context, creds *OAuthCredentials) (*OAuthCredentials, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	if p.oauthUrls.TokenUrl == "" {
		return nil, errors.New("token URL is empty")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", creds.ClientID)
	form.Set("refresh_token", creds.RefreshToken)

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if p.userAgent != "" {
		header.Set("Useragent", p.userAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.oauthUrls.TokenUrl,
		Headers: header,
		Body:    []byte(form.Encode()),
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		var tokenErr TokenError
		if err := json.Unmarshal(resp.Body, &tokenErr); err == nil && tokenErr.Error != "" {
			return nil, fmt.Errorf("token refresh failed: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
		}

		return nil, errors.New("token refresh response missing access_token")
	}

	now := time.Now()

	updated := *creds
	updated.AccessToken = tokenResp.AccessToken
	updated.TokenType = tokenResp.TokenType

	if tokenResp.RefreshToken != "" {
		updated.RefreshToken = tokenResp.RefreshToken
	}

	if tokenResp.Scope != "" {
		updated.Scopes = strings.Fields(tokenResp.Scope)
	}

	if tokenResp.ExpiresIn > 0 {
		updated.ExpiresAt = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	log.Debug(ctx, "oauth token refreshed", log.String("expires_at", updated.ExpiresAt.Format(time.RFC3339)))

	return &updated, nil
}
