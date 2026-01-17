package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	AuthorizeURL = "https://auth.openai.com/oauth/authorize"
	//nolint:gosec // false alert.
	TokenURL    = "https://auth.openai.com/oauth/token"
	ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	RedirectURI = "http://localhost:1455/auth/callback"
	Scopes      = "openid profile email offline_access"

	// UA keep consistent with Codex CLI.
	UA = "codex_cli_rs/0.38.0 (Ubuntu 22.04.0; x86_64) WindowsTerminal"
)

type OAuth2Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ClientID     string    `json:"client_id,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
}

type TokenResponse struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

type TokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func ParseCredentialsJSON(raw string) (*OAuth2Credentials, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("empty credentials")
	}

	var creds OAuth2Credentials
	if err := json.Unmarshal([]byte(trimmed), &creds); err != nil {
		return nil, err
	}

	if creds.AccessToken == "" {
		return nil, errors.New("access_token is empty")
	}

	if creds.ClientID == "" {
		creds.ClientID = ClientID
	}

	if creds.AccountID == "" {
		creds.AccountID = ExtractChatGPTAccountIDFromJWT(creds.AccessToken)
	}

	// If refresh_token exists but expires_at is missing, assume 1 hour.
	if creds.RefreshToken != "" && creds.ExpiresAt.IsZero() {
		creds.ExpiresAt = time.Now().Add(1 * time.Hour)
	}

	return &creds, nil
}

func (c *OAuth2Credentials) IsExpired(now time.Time) bool {
	if c == nil {
		return true
	}

	if c.ExpiresAt.IsZero() {
		return true
	}

	// Consider token expired 3 minutes earlier.
	return now.Add(3 * time.Minute).After(c.ExpiresAt)
}

func (c *OAuth2Credentials) ToJSON() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func ExtractChatGPTAccountIDFromJWT(tokenStr string) string {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())

	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return ""
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	authClaims, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return ""
	}

	accountID, ok := authClaims["chatgpt_account_id"].(string)
	if !ok {
		return ""
	}

	return accountID
}

type TokenURLs struct {
	Authorize string
	Token     string
}

// DefaultTokenURLs are the production OpenAI OAuth endpoints.
var DefaultTokenURLs = TokenURLs{
	Authorize: AuthorizeURL,
	Token:     TokenURL,
}

func (c *OAuth2Credentials) Refresh(ctx context.Context, hc *httpclient.HttpClient) (*OAuth2Credentials, error) {
	if c == nil {
		return nil, errors.New("nil credentials")
	}

	if c.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	if c.ClientID == "" {
		c.ClientID = ClientID
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.ClientID)
	form.Set("refresh_token", c.RefreshToken)

	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    DefaultTokenURLs.Token,

		Headers: http.Header{
			"Content-Type": []string{"application/x-www-form-urlencoded"},
			"User-Agent":   []string{UA},
			"Accept":       []string{"application/json"},
		},
		Body: []byte(form.Encode()),
	}

	resp, err := hc.Do(ctx, req)
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

	updated := *c
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

	updated.AccountID = ExtractChatGPTAccountIDFromJWT(updated.AccessToken)

	log.Debug(ctx, "codex token refreshed", log.String("expires_at", updated.ExpiresAt.Format(time.RFC3339)))

	return &updated, nil
}
