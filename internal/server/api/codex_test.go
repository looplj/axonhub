package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCodexHandlers_StartOAuth_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewCodexHandlers(CodexHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClient(),
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithProjectID(c.Request.Context(), 123))
		c.Next()
	})
	router.POST("/admin/codex/oauth/start", h.StartOAuth)

	req := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/start", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid request format")
}

func TestCodexHandlers_Exchange_StateDeletedOnTokenExchangeFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var tokenCalls int
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"bad_gateway"}`))
	}))
	t.Cleanup(tokenServer.Close)

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == codexTokenURL {
			proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, tokenServer.URL, req.Body)
			if err != nil {
				return nil, err
			}
			proxyReq.Header = req.Header.Clone()
			return http.DefaultTransport.RoundTrip(proxyReq)
		}

		return http.DefaultTransport.RoundTrip(req)
	})

	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCodexHandlers(CodexHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithProjectID(c.Request.Context(), 123))
		c.Next()
	})
	router.POST("/admin/codex/oauth/start", h.StartOAuth)
	router.POST("/admin/codex/oauth/exchange", h.Exchange)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCodexOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))
	require.NotEmpty(t, startResp.SessionID)

	exchangeBody, err := json.Marshal(ExchangeCodexOAuthRequest{
		SessionID:   startResp.SessionID,
		CallbackURL: "http://localhost:1455/auth/callback?code=test-code&state=" + startResp.SessionID,
	})
	require.NoError(t, err)

	exchangeReq := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/exchange", bytes.NewBuffer(exchangeBody))
	exchangeReq.Header.Set("Content-Type", "application/json")
	exchangeW := httptest.NewRecorder()
	router.ServeHTTP(exchangeW, exchangeReq)
	require.Equal(t, http.StatusBadGateway, exchangeW.Code)
	require.Equal(t, 1, tokenCalls)

	exchangeReq2 := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/exchange", bytes.NewBuffer(exchangeBody))
	exchangeReq2.Header.Set("Content-Type", "application/json")
	exchangeW2 := httptest.NewRecorder()
	router.ServeHTTP(exchangeW2, exchangeReq2)
	require.Equal(t, http.StatusBadRequest, exchangeW2.Code)
	require.Contains(t, exchangeW2.Body.String(), "invalid or expired oauth session")
}

func TestCodexHandlers_Exchange_RejectsStateMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(tokenServer.Close)

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == codexTokenURL {
			body, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()
			proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, tokenServer.URL, bytes.NewBuffer(body))
			if err != nil {
				return nil, err
			}
			proxyReq.Header = req.Header.Clone()
			return http.DefaultTransport.RoundTrip(proxyReq)
		}

		return http.DefaultTransport.RoundTrip(req)
	})

	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCodexHandlers(CodexHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithProjectID(c.Request.Context(), 123))
		c.Next()
	})
	router.POST("/admin/codex/oauth/start", h.StartOAuth)
	router.POST("/admin/codex/oauth/exchange", h.Exchange)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCodexOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))
	require.NotEmpty(t, startResp.SessionID)

	exchangeBody, err := json.Marshal(ExchangeCodexOAuthRequest{
		SessionID:   startResp.SessionID,
		CallbackURL: "http://localhost:1455/auth/callback?code=test-code&state=mismatch",
	})
	require.NoError(t, err)

	exchangeReq := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/exchange", bytes.NewBuffer(exchangeBody))
	exchangeReq.Header.Set("Content-Type", "application/json")
	exchangeW := httptest.NewRecorder()
	router.ServeHTTP(exchangeW, exchangeReq)
	require.Equal(t, http.StatusBadRequest, exchangeW.Code)
	require.Contains(t, exchangeW.Body.String(), "oauth state mismatch")
	exchangeBody2, err := json.Marshal(ExchangeCodexOAuthRequest{
		SessionID:   startResp.SessionID,
		CallbackURL: "http://localhost:1455/auth/callback?code=test-code&state=" + startResp.SessionID,
	})
	require.NoError(t, err)

	exchangeReq2 := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/exchange", bytes.NewBuffer(exchangeBody2))
	exchangeReq2.Header.Set("Content-Type", "application/json")
	exchangeW2 := httptest.NewRecorder()
	router.ServeHTTP(exchangeW2, exchangeReq2)
	require.Equal(t, http.StatusBadRequest, exchangeW2.Code)
	require.Contains(t, exchangeW2.Body.String(), "invalid or expired oauth session")
}
func TestCodexHandlers_Exchange_DeletesStateOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(tokenServer.Close)

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == codexTokenURL {
			body, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()
			proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, tokenServer.URL, bytes.NewBuffer(body))
			if err != nil {
				return nil, err
			}
			proxyReq.Header = req.Header.Clone()
			return http.DefaultTransport.RoundTrip(proxyReq)
		}

		return http.DefaultTransport.RoundTrip(req)
	})

	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCodexHandlers(CodexHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithProjectID(c.Request.Context(), 123))
		c.Next()
	})
	router.POST("/admin/codex/oauth/start", h.StartOAuth)
	router.POST("/admin/codex/oauth/exchange", h.Exchange)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCodexOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	exchangeBody, err := json.Marshal(ExchangeCodexOAuthRequest{
		SessionID:   startResp.SessionID,
		CallbackURL: "http://localhost:1455/auth/callback?code=test-code&state=" + startResp.SessionID,
	})
	require.NoError(t, err)

	exchangeReq := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/exchange", bytes.NewBuffer(exchangeBody))
	exchangeReq.Header.Set("Content-Type", "application/json")
	exchangeW := httptest.NewRecorder()
	router.ServeHTTP(exchangeW, exchangeReq)
	require.Equal(t, http.StatusOK, exchangeW.Code)

	exchangeReq2 := httptest.NewRequest(http.MethodPost, "/admin/codex/oauth/exchange", bytes.NewBuffer(exchangeBody))
	exchangeReq2.Header.Set("Content-Type", "application/json")
	exchangeW2 := httptest.NewRecorder()
	router.ServeHTTP(exchangeW2, exchangeReq2)
	require.Equal(t, http.StatusBadRequest, exchangeW2.Code)
	require.Contains(t, exchangeW2.Body.String(), "invalid or expired oauth session")
}
