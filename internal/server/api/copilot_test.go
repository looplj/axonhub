package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

// fakeClock is a test implementation of Clock that allows advancing time.
type fakeClock struct {
	currentTime time.Time
	channels    []chan time.Time
}

func (fc *fakeClock) Now() time.Time {
	return fc.currentTime
}

func (fc *fakeClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	fc.channels = append(fc.channels, ch)

	return ch
}

func (fc *fakeClock) Advance(d time.Duration) {
	fc.currentTime = fc.currentTime.Add(d)
	for _, ch := range fc.channels {
		select {
		case ch <- fc.currentTime:
		default:
		}
	}

	fc.channels = nil
}

func TestCopilotHandlers_StartOAuth_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)
		require.Equal(t, defaultGithubCopilotClientID, r.FormValue("client_id"))
		require.Equal(t, githubCopilotScope, r.FormValue("scope"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code-123",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL: deviceCodeServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)

	req := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.SessionID)
	require.Equal(t, "ABCD-EFGH", resp.UserCode)
	require.Equal(t, "https://github.com/login/device", resp.VerificationURI)
	require.Equal(t, 900, resp.ExpiresIn)
	require.Equal(t, 5, resp.Interval)
}

func TestCopilotHandlers_StartOAuth_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClient(),
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)

	req := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid request format")
}

func TestCopilotHandlers_StartOAuth_GitHubError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"service_unavailable"}`))
	}))
	defer deviceCodeServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL: deviceCodeServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)

	req := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
}

func TestCopilotHandlers_StartOAuth_EmptyDeviceCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL: deviceCodeServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)

	req := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	require.Contains(t, w.Body.String(), "device code not received")
}

func TestCopilotHandlers_StartOAuth_WithProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "WXYZ-1234",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL: deviceCodeServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)

	reqBody, _ := json.Marshal(StartCopilotOAuthRequest{})

	req := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestCopilotHandlers_PollOAuth_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		err := r.ParseForm()
		require.NoError(t, err)
		require.Equal(t, defaultGithubCopilotClientID, r.FormValue("client_id"))
		require.Equal(t, "test-device-code", r.FormValue("device_code"))
		require.Equal(t, deviceGrantType, r.FormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "gho_test_access_token",
			"token_type": "bearer",
			"scope": "read:user"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusOK, pollW.Code)

	var pollResp PollCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(pollW.Body.Bytes(), &pollResp))
	require.Equal(t, "complete", pollResp.Status)
	require.Equal(t, "gho_test_access_token", pollResp.Token)
	require.Equal(t, "bearer", pollResp.Type)
	require.Equal(t, "read:user", pollResp.Scope)
}

func TestCopilotHandlers_PollOAuth_AuthorizationPending(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error": "authorization_pending",
			"error_description": "The authorization request is still pending"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusOK, pollW.Code)

	var pollResp PollCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(pollW.Body.Bytes(), &pollResp))
	require.Equal(t, "pending", pollResp.Status)
	require.Contains(t, pollResp.Message, "Authorization pending")
}

func TestCopilotHandlers_PollOAuth_SlowDown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error": "slow_down",
			"error_description": "Polling too fast"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusOK, pollW.Code)

	var pollResp PollCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(pollW.Body.Bytes(), &pollResp))
	require.Equal(t, "slow_down", pollResp.Status)
	require.Contains(t, pollResp.Message, "slow down")
}

func TestCopilotHandlers_PollOAuth_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error": "expired_token",
			"error_description": "The device code has expired"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadRequest, pollW.Code)
	require.Contains(t, pollW.Body.String(), "device code expired")
}

func TestCopilotHandlers_PollOAuth_AccessDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error": "access_denied",
			"error_description": "User denied the authorization"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadRequest, pollW.Code)
	require.Contains(t, pollW.Body.String(), "access denied")
}

func TestCopilotHandlers_PollOAuth_InvalidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClient(),
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: "invalid-session-id",
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadRequest, pollW.Code)
	require.Contains(t, pollW.Body.String(), "invalid or expired session")
}

func TestCopilotHandlers_PollOAuth_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClient(),
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBufferString("{"))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadRequest, pollW.Code)
	require.Contains(t, pollW.Body.String(), "invalid request format")
}

func TestCopilotHandlers_PollOAuth_MissingSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClient(),
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: "",
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadRequest, pollW.Code)
}

func TestCopilotHandlers_PollOAuth_DeviceCodeExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 2,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL: deviceCodeServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	// Create a fake clock that can be advanced for testing
	fakeClock := &fakeClock{
		currentTime: time.Now(),
	}

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
		Clock:       fakeClock,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))
	// Advance the fake clock by 6 seconds to simulate session expiry
	fakeClock.Advance(6 * time.Second)

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadRequest, pollW.Code)
	require.Contains(t, pollW.Body.String(), "device code expired")
}

func TestCopilotHandlers_PollOAuth_FormEncodedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		_, _ = w.Write([]byte(`access_token=gho_form_encoded_token&token_type=bearer&scope=read:user`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusOK, pollW.Code)

	var pollResp PollCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(pollW.Body.Bytes(), &pollResp))
	require.Equal(t, "complete", pollResp.Status)
	require.Equal(t, "gho_form_encoded_token", pollResp.Token)
}

func TestCopilotHandlers_PollOAuth_DeletesStateOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "gho_test_token",
			"token_type": "bearer",
			"scope": "read:user"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusOK, pollW.Code)

	pollReq2 := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq2.Header.Set("Content-Type", "application/json")
	pollW2 := httptest.NewRecorder()
	router.ServeHTTP(pollW2, pollReq2)

	require.Equal(t, http.StatusBadRequest, pollW2.Code)
	require.Contains(t, pollW2.Body.String(), "invalid or expired session")
}

func TestCopilotHandlers_PollOAuth_UnknownError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error": "unknown_error",
			"error_description": "Something went wrong"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusBadGateway, pollW.Code)
	require.Contains(t, pollW.Body.String(), "OAuth error")
}

func TestCopilotHandlers_PollOAuth_WithProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deviceCodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "test-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5
		}`))
	}))
	defer deviceCodeServer.Close()

	accessTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "gho_test_token",
			"token_type": "bearer",
			"scope": "read:user"
		}`))
	}))
	defer accessTokenServer.Close()

	transport := &testCopilotTransport{
		deviceCodeURL:  deviceCodeServer.URL,
		accessTokenURL: accessTokenServer.URL,
	}
	hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	h := NewCopilotHandlers(CopilotHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  hc,
	})

	router := gin.New()
	router.POST("/admin/copilot/oauth/start", h.StartOAuth)
	router.POST("/admin/copilot/oauth/poll", h.PollOAuth)

	startReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/start", bytes.NewBufferString("{}"))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	var startResp StartCopilotOAuthResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &startResp))

	pollBody, _ := json.Marshal(PollCopilotOAuthRequest{
		SessionID: startResp.SessionID,
	})
	pollReq := httptest.NewRequest(http.MethodPost, "/admin/copilot/oauth/poll", bytes.NewBuffer(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollW := httptest.NewRecorder()
	router.ServeHTTP(pollW, pollReq)

	require.Equal(t, http.StatusOK, pollW.Code)
}

type testCopilotTransport struct {
	deviceCodeURL  string
	accessTokenURL string
}

func (t *testCopilotTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.String(), "github.com/login/device/code") {
		if t.deviceCodeURL != "" {
			body, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()

			proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, t.deviceCodeURL, bytes.NewBuffer(body))
			if err != nil {
				return nil, err
			}
			proxyReq.Header = req.Header.Clone()
			return http.DefaultTransport.RoundTrip(proxyReq)
		}
	}

	if strings.Contains(req.URL.String(), "github.com/login/oauth/access_token") {
		if t.accessTokenURL != "" {
			body, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()

			proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, t.accessTokenURL, bytes.NewBuffer(body))
			if err != nil {
				return nil, err
			}
			proxyReq.Header = req.Header.Clone()
			return http.DefaultTransport.RoundTrip(proxyReq)
		}
	}

	return nil, fmt.Errorf("unexpected request to %s", req.URL)
}
