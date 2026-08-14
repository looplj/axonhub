package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/xai/subscription"
)

func TestXAIHandlers_StartOAuth_returns_official_PKCE_URL(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	handler := NewXAIHandlers(XAIHandlersParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, HttpClient: httpclient.NewHttpClient()})
	router := gin.New()
	withXAITestUser(router, 1)
	router.POST("/admin/xai/oauth/start", handler.StartOAuth)

	// When
	request := httptest.NewRequest(http.MethodPost, "/admin/xai/oauth/start", bytes.NewBufferString("{}"))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	// Then
	require.Equal(t, http.StatusOK, recorder.Code)
	var response StartXAIOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	parsed, err := url.Parse(response.AuthURL)
	require.NoError(t, err)
	require.Equal(t, subscription.AuthorizeURL, parsed.Scheme+"://"+parsed.Host+parsed.Path)
	require.Equal(t, subscription.ClientID, parsed.Query().Get("client_id"))
	require.Equal(t, subscription.Scopes, parsed.Query().Get("scope"))
	require.Equal(t, "generic", parsed.Query().Get("plan"))
	require.NotEmpty(t, parsed.Query().Get("nonce"))
	require.Equal(t, response.SessionID, parsed.Query().Get("state"))
	state, err := handler.stateCache.Get(t.Context(), xaiOAuthCacheKey(response.SessionID))
	require.NoError(t, err)
	require.NotEmpty(t, state.CodeVerifier)
	require.Equal(t, 1, state.UserID)
}

func TestXAIHandlers_Exchange_returns_normalized_credentials(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, subscription.TokenURL, request.URL.String())
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "authorization_code", values.Get("grant_type"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`)),
		}, nil
	})
	handler := NewXAIHandlers(XAIHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}),
	})
	router := gin.New()
	withXAITestUser(router, 1)
	router.POST("/admin/xai/oauth/start", handler.StartOAuth)
	router.POST("/admin/xai/oauth/exchange", handler.Exchange)
	start := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/admin/xai/oauth/start", bytes.NewBufferString("{}"))
	startRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(start, startRequest)
	var session StartXAIOAuthResponse
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &session))
	payload, err := json.Marshal(ExchangeXAIOAuthRequest{
		SessionID:   session.SessionID,
		CallbackURL: subscription.RedirectURI + "?code=synthetic-code&state=" + session.SessionID,
	})
	require.NoError(t, err)

	// When
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/xai/oauth/exchange", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	// Then
	require.Equal(t, http.StatusOK, recorder.Code)
	var response ExchangeXAIOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	credentials, err := oauth.ParseCredentialsJSON(response.Credentials)
	require.NoError(t, err)
	require.Equal(t, "access", credentials.AccessToken)
	require.Equal(t, subscription.ClientID, credentials.ClientID)
}

func TestXAIHandlers_DecodeSSO_returns_normalized_credentials(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	transport := roundTripperFunc(newXAISSORoundTripper(t))
	handler := NewXAIHandlers(XAIHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}),
	})
	router := gin.New()
	withXAITestUser(router, 1)
	router.POST("/admin/xai/oauth/sso", handler.DecodeSSO)

	// When
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/xai/oauth/sso", bytes.NewBufferString(`{"sso_token":"sso=synthetic-sso"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	// Then
	require.Equal(t, http.StatusOK, recorder.Code)
	var response DecodeXAISSOResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	credentials, err := oauth.ParseCredentialsJSON(response.Credentials)
	require.NoError(t, err)
	require.Equal(t, "access", credentials.AccessToken)
}

func TestXAIHandlers_Exchange_rejects_session_from_another_user(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	handler := NewXAIHandlers(XAIHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient: httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected token request: %s", request.URL)
			return nil, nil
		})}),
	})
	startRouter := gin.New()
	withXAITestUser(startRouter, 1)
	startRouter.POST("/admin/xai/oauth/start", handler.StartOAuth)
	start := httptest.NewRecorder()
	startRouter.ServeHTTP(start, httptest.NewRequest(http.MethodPost, "/admin/xai/oauth/start", nil))
	var session StartXAIOAuthResponse
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &session))
	payload, err := json.Marshal(ExchangeXAIOAuthRequest{
		SessionID:   session.SessionID,
		CallbackURL: subscription.RedirectURI + "?code=synthetic-code&state=" + session.SessionID,
	})
	require.NoError(t, err)
	exchangeRouter := gin.New()
	withXAITestUser(exchangeRouter, 2)
	exchangeRouter.POST("/admin/xai/oauth/exchange", handler.Exchange)

	// When
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/xai/oauth/exchange", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	exchangeRouter.ServeHTTP(recorder, request)

	// Then
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, err = handler.stateCache.Get(t.Context(), xaiOAuthCacheKey(session.SessionID))
	require.NoError(t, err)
}

func withXAITestUser(router *gin.Engine, userID int) {
	router.Use(func(c *gin.Context) {
		ctx := contexts.WithUser(c.Request.Context(), &ent.User{ID: userID})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
}

func newXAISSORoundTripper(t *testing.T) func(*http.Request) (*http.Response, error) {
	t.Helper()
	return func(request *http.Request) (*http.Response, error) {
		response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}
		switch request.URL.String() {
		case subscription.SSOAccountsURL:
			require.Contains(t, request.Header.Get("Cookie"), "sso=synthetic-sso")
		case subscription.SSODeviceURL:
			response.Body = io.NopCloser(strings.NewReader(`{"device_code":"device","user_code":"user","verification_uri_complete":"https://auth.x.ai/oauth2/device/verify?user_code=user","interval":1,"expires_in":60}`))
		case "https://auth.x.ai/oauth2/device/verify?user_code=user", "https://auth.x.ai/oauth2/device/consent", "https://auth.x.ai/oauth2/device/done":
		case subscription.SSOVerifyURL:
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://auth.x.ai/oauth2/device/consent")
		case subscription.SSOApproveURL:
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://auth.x.ai/oauth2/device/done")
		case subscription.TokenURL:
			response.Body = io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`))
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		return response, nil
	}
}
