package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/llm/httpclient"
)

// ExchangeStrategy defines the interface for different OAuth token exchange formats.
// Different providers may use different request formats (form-encoded vs JSON).
type ExchangeStrategy interface {
	// BuildExchangeRequest builds the HTTP request for exchanging an authorization code
	BuildExchangeRequest(params ExchangeParams, tokenURL string) (*httpclient.Request, error)
	// BuildRefreshRequest builds the HTTP request for refreshing a token
	BuildRefreshRequest(creds *OAuthCredentials, tokenURL string) (*httpclient.Request, error)
}

// FormEncodedStrategy implements OAuth using form-urlencoded requests (standard OAuth2).
type FormEncodedStrategy struct {
	UserAgent string
}

// JSONStrategy implements OAuth using JSON requests (Claude Code style).
type JSONStrategy struct {
	UserAgent string
}

// BuildExchangeRequest implements ExchangeStrategy for form-encoded requests.
func (s *FormEncodedStrategy) BuildExchangeRequest(params ExchangeParams, tokenURL string) (*httpclient.Request, error) {
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

// BuildRefreshRequest implements ExchangeStrategy for form-encoded requests.
func (s *FormEncodedStrategy) BuildRefreshRequest(creds *OAuthCredentials, tokenURL string) (*httpclient.Request, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", creds.ClientID)
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

// BuildExchangeRequest implements ExchangeStrategy for JSON requests.
func (s *JSONStrategy) BuildExchangeRequest(params ExchangeParams, tokenURL string) (*httpclient.Request, error) {
	reqBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          params.Code,
		"client_id":     params.ClientID,
		"redirect_uri":  params.RedirectURI,
		"code_verifier": params.CodeVerifier,
	}

	// Claude requires state in the token exchange
	if params.State != "" {
		reqBody["state"] = params.State
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal exchange request: %w", err)
	}

	header := http.Header{
		"Content-Type": []string{"application/json"},
		"Accept":       []string{"application/json"},
	}
	if s.UserAgent != "" {
		header.Set("User-Agent", s.UserAgent)
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     tokenURL,
		Headers: header,
		Body:    bodyBytes,
	}, nil
}

// BuildRefreshRequest implements ExchangeStrategy for JSON requests.
func (s *JSONStrategy) BuildRefreshRequest(creds *OAuthCredentials, tokenURL string) (*httpclient.Request, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     creds.ClientID,
		"refresh_token": creds.RefreshToken,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal refresh request: %w", err)
	}

	header := http.Header{
		"Content-Type": []string{"application/json"},
		"Accept":       []string{"application/json"},
	}
	if s.UserAgent != "" {
		header.Set("User-Agent", s.UserAgent)
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     tokenURL,
		Headers: header,
		Body:    bodyBytes,
	}, nil
}

// ParseTokenResponse parses the OAuth token response and returns credentials.
func ParseTokenResponse(respBody []byte, clientID string) (*OAuthCredentials, error) {
	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		var tokenErr TokenError
		if err := json.Unmarshal(respBody, &tokenErr); err == nil && tokenErr.Error != "" {
			return nil, fmt.Errorf("token request failed: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
		}
		return nil, errors.New("token response missing access_token")
	}

	creds := &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     clientID,
		TokenType:    tokenResp.TokenType,
	}

	if tokenResp.Scope != "" {
		creds.Scopes = strings.Fields(tokenResp.Scope)
	}

	if tokenResp.ExpiresIn > 0 {
		creds.ExpiresAt = tokenResp.ExpiresAt()
	}

	return creds, nil
}
