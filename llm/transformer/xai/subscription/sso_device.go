package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm/oauth"
)

const (
	ssoMaxResponseBody = 2 << 20
	ssoTimeout         = 90 * time.Second
)

type SSODeviceOptions struct {
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

type ssoDeviceFlow struct {
	client *http.Client
	jar    http.CookieJar
	sleep  func(context.Context, time.Duration) error
}

type ssoDeviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

type ssoTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	IDToken          string `json:"id_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func ConvertSSOToBuild(ctx context.Context, rawToken string, options SSODeviceOptions) (*oauth.OAuthCredentials, error) {
	token := NormalizeSSOToken(rawToken)
	if token == "" {
		return nil, errors.New("xAI SSO token is required")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create xAI SSO cookie jar: %w", err)
	}
	seedSSOCookies(jar, token)

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	clientCopy := *client
	clientCopy.Timeout = ssoTimeout
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	sleep := options.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	return (&ssoDeviceFlow{client: &clientCopy, jar: jar, sleep: sleep}).convert(ctx)
}

func (flow *ssoDeviceFlow) convert(ctx context.Context) (*oauth.OAuthCredentials, error) {
	status, finalURL, _, err := flow.do(ctx, http.MethodGet, SSOAccountsURL, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || strings.Contains(finalURL, "sign-in") || strings.Contains(finalURL, "sign-up") {
		return nil, errors.New("xAI SSO token is unauthorized")
	}
	if status < 200 || status >= 400 {
		return nil, fmt.Errorf("validate xAI SSO: HTTP %d", status)
	}

	status, _, body, err := flow.do(ctx, http.MethodPost, SSODeviceURL, url.Values{"client_id": {ClientID}, "scope": {SSOScopes}})
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("start xAI device flow: HTTP %d", status)
	}
	var device ssoDeviceResponse
	if err := json.Unmarshal(body, &device); err != nil {
		return nil, fmt.Errorf("parse xAI device flow response: %w", err)
	}
	if device.DeviceCode == "" || device.UserCode == "" || !trustedXAIURL(device.VerificationURIComplete) {
		return nil, errors.New("xAI device flow response is incomplete")
	}
	if device.Interval <= 0 {
		device.Interval = 5
	}
	if device.ExpiresIn <= 0 {
		device.ExpiresIn = 1800
	}

	if status, _, _, err = flow.do(ctx, http.MethodGet, device.VerificationURIComplete, nil); err != nil {
		return nil, fmt.Errorf("open xAI device verification: %w", err)
	}
	if status < 200 || status >= 400 {
		return nil, fmt.Errorf("open xAI device verification: HTTP %d", status)
	}
	status, finalURL, _, err = flow.do(ctx, http.MethodPost, SSOVerifyURL, url.Values{"user_code": {device.UserCode}})
	if err != nil {
		return nil, fmt.Errorf("verify xAI device code: %w", err)
	}
	if status < 200 || status >= 400 {
		return nil, fmt.Errorf("verify xAI device code: HTTP %d", status)
	}
	if !strings.Contains(finalURL, "consent") {
		return nil, errors.New("verify xAI device code did not reach consent")
	}
	status, finalURL, _, err = flow.do(ctx, http.MethodPost, SSOApproveURL, url.Values{
		"user_code": {device.UserCode}, "action": {"allow"}, "principal_type": {"User"}, "principal_id": {""},
	})
	if err != nil {
		return nil, fmt.Errorf("approve xAI device code: %w", err)
	}
	if status < 200 || status >= 400 {
		return nil, fmt.Errorf("approve xAI device code: HTTP %d", status)
	}
	if !strings.Contains(finalURL, "done") {
		return nil, errors.New("approve xAI device code did not reach done")
	}

	return flow.pollToken(ctx, device)
}

func (flow *ssoDeviceFlow) pollToken(ctx context.Context, device ssoDeviceResponse) (*oauth.OAuthCredentials, error) {
	interval := time.Duration(device.Interval) * time.Second
	deadline := time.Now().Add(min(time.Duration(device.ExpiresIn)*time.Second, 75*time.Second))
	for time.Now().Before(deadline) {
		sleepFor := min(interval, time.Until(deadline))
		if err := flow.sleep(ctx, sleepFor); err != nil {
			return nil, err
		}
		status, _, body, err := flow.do(ctx, http.MethodPost, TokenURL, url.Values{
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {ClientID}, "device_code": {device.DeviceCode},
		})
		if err != nil {
			return nil, err
		}
		var token ssoTokenResponse
		if err := json.Unmarshal(body, &token); err != nil {
			return nil, fmt.Errorf("parse xAI device token response: %w", err)
		}
		if status >= 200 && status < 300 && token.AccessToken != "" {
			if token.ExpiresIn <= 0 {
				token.ExpiresIn = int64((6 * time.Hour).Seconds())
			}
			if strings.TrimSpace(token.TokenType) == "" {
				token.TokenType = "Bearer"
			}
			return &oauth.OAuthCredentials{
				AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, IDToken: token.IDToken,
				ClientID: ClientID, TokenType: token.TokenType, Scopes: strings.Fields(token.Scope), ExpiresAt: time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
			}, nil
		}
		switch token.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "access_denied", "expired_token":
			return nil, fmt.Errorf("xAI device authorization failed: %s", token.Error)
		default:
			return nil, fmt.Errorf("xAI device token polling failed: %s", strings.TrimSpace(token.ErrorDescription+" "+token.Error))
		}
	}
	return nil, errors.New("xAI device token polling timed out")
}

func (flow *ssoDeviceFlow) do(ctx context.Context, method, endpoint string, form url.Values) (int, string, []byte, error) {
	currentURL, currentMethod, currentForm := endpoint, method, form
	for range 9 {
		if !trustedXAIURL(currentURL) {
			return 0, currentURL, nil, errors.New("xAI OAuth URL is not trusted")
		}
		var body io.Reader
		if currentForm != nil {
			body = strings.NewReader(currentForm.Encode())
		}
		request, err := http.NewRequestWithContext(ctx, currentMethod, currentURL, body)
		if err != nil {
			return 0, currentURL, nil, err
		}
		request.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.8")
		request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		request.Header.Set("Accept-Language", "en-US,en;q=0.9")
		if currentForm != nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		flow.applyCookies(request)
		response, err := flow.client.Do(request)
		if err != nil {
			return 0, currentURL, nil, err
		}
		flow.jar.SetCookies(request.URL, response.Cookies())
		data, readErr := io.ReadAll(io.LimitReader(response.Body, ssoMaxResponseBody+1))
		_ = response.Body.Close()
		if readErr != nil {
			return response.StatusCode, currentURL, nil, readErr
		}
		if len(data) > ssoMaxResponseBody {
			return response.StatusCode, currentURL, nil, errors.New("xAI OAuth response exceeds 2 MiB")
		}
		if response.StatusCode < 300 || response.StatusCode > 399 {
			return response.StatusCode, currentURL, data, nil
		}
		location, err := response.Location()
		if err != nil {
			return response.StatusCode, currentURL, data, fmt.Errorf("resolve xAI OAuth redirect: %w", err)
		}
		currentURL = request.URL.ResolveReference(location).String()
		if response.StatusCode == http.StatusSeeOther || ((response.StatusCode == http.StatusMovedPermanently || response.StatusCode == http.StatusFound) && currentMethod != http.MethodGet) {
			currentMethod, currentForm = http.MethodGet, nil
		}
	}
	return 0, currentURL, nil, errors.New("xAI OAuth redirected too many times")
}

func seedSSOCookies(jar http.CookieJar, token string) {
	for _, endpoint := range []string{SSOAccountsURL, "https://auth.x.ai/"} {
		target, _ := url.Parse(endpoint)
		jar.SetCookies(target, []*http.Cookie{{Name: "sso", Value: token, Path: "/", Secure: true, HttpOnly: true}, {Name: "sso-rw", Value: token, Path: "/", Secure: true, HttpOnly: true}})
	}
}

func (flow *ssoDeviceFlow) applyCookies(request *http.Request) {
	for _, cookie := range flow.jar.Cookies(request.URL) {
		request.AddCookie(cookie)
	}
}

func trustedXAIURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "x.ai" || strings.HasSuffix(host, ".x.ai")
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
