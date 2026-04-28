package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// mockTokenExchanger is a mock implementation of TokenExchanger for testing.
type mockTokenExchanger struct {
	token     string
	expiresAt int64
	err       error
}

func (m *mockTokenExchanger) Exchange(ctx context.Context, accessToken string) (string, int64, error) {
	return m.token, m.expiresAt, m.err
}

func (m *mockTokenExchanger) ExchangeWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (string, int64, error) {
	return m.token, m.expiresAt, m.err
}

// Test DeviceFlowProvider construction.
func TestNewDeviceFlowProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params DeviceFlowProviderParams
	}{
		{
			name:   "empty params",
			params: DeviceFlowProviderParams{},
		},
		{
			name: "with config",
			params: DeviceFlowProviderParams{
				Config: DeviceFlowConfig{
					DeviceAuthURL: "https://example.com/device",
					TokenURL:      "https://example.com/token",
					ClientID:      "test-client",
				},
			},
		},
		{
			name: "with credentials",
			params: DeviceFlowProviderParams{
				Credentials: &OAuthCredentials{
					AccessToken: "test-token",
					ExpiresAt:   time.Now().Add(time.Hour),
				},
			},
		},
		{
			name: "with token exchanger",
			params: DeviceFlowProviderParams{
				TokenExchanger: &mockTokenExchanger{token: "exchanged-token", expiresAt: time.Now().Add(time.Hour).Unix()},
			},
		},
		{
			name: "with onRefreshed callback",
			params: DeviceFlowProviderParams{
				OnRefreshed: func(ctx context.Context, refreshed *OAuthCredentials) error {
					return nil
				},
			},
		},
		{
			name: "with all fields",
			params: DeviceFlowProviderParams{
				Config: DeviceFlowConfig{
					DeviceAuthURL: "https://example.com/device",
					TokenURL:      "https://example.com/token",
					ClientID:      "test-client",
					Scopes:        []string{"read", "write"},
					UserAgent:     "test-agent",
				},
				HTTPClient: httpclient.NewHttpClient(),
				Credentials: &OAuthCredentials{
					AccessToken:  "access-token",
					RefreshToken: "refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				},
				TokenExchanger: &mockTokenExchanger{token: "exchanged"},
				OnRefreshed: func(ctx context.Context, refreshed *OAuthCredentials) error {
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewDeviceFlowProvider(tt.params)
			require.NotNil(t, provider)
			assert.Equal(t, tt.params.Config, provider.config)
		})
	}
}

// Test Start() validation errors.
func TestDeviceFlowProvider_Start_Validation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name    string
		params  DeviceFlowProviderParams
		wantErr string
	}{
		{
			name:    "nil http client",
			params:  DeviceFlowProviderParams{},
			wantErr: "http client is nil",
		},
		{
			name: "empty device auth URL",
			params: DeviceFlowProviderParams{
				HTTPClient: httpclient.NewHttpClient(),
			},
			wantErr: "device authorization URL is empty",
		},
		{
			name: "empty client ID",
			params: DeviceFlowProviderParams{
				HTTPClient: httpclient.NewHttpClient(),
				Config: DeviceFlowConfig{
					DeviceAuthURL: "https://example.com/device",
				},
			},
			wantErr: "client_id is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewDeviceFlowProvider(tt.params)
			_, err := provider.Start(ctx)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

// Test Start() success.
func TestDeviceFlowProvider_Start_Success(t *testing.T) {
	t.Parallel()

	var (
		gotUA   string
		gotForm url.Values
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/device", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		gotUA = r.Header.Get("User-Agent")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotForm, err = url.ParseQuery(string(body))
		require.NoError(t, err)

		resp := DeviceFlowResponse{
			DeviceCode:      "device-code-123",
			UserCode:        "USER-CODE-456",
			VerificationURI: "https://example.com/verify",
			CompleteURI:     "https://example.com/verify?code=USER-CODE-456",
			ExpiresIn:       900,
			Interval:        5,
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "test-client",
			Scopes:        []string{"read:user", "repo"},
			UserAgent:     "axonhub-test",
		},
	})

	ctx := context.Background()
	resp, err := provider.Start(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "device-code-123", resp.DeviceCode)
	assert.Equal(t, "USER-CODE-456", resp.UserCode)
	assert.Equal(t, "https://example.com/verify", resp.VerificationURI)
	assert.Equal(t, "https://example.com/verify?code=USER-CODE-456", resp.CompleteURI)
	assert.Equal(t, 900, resp.ExpiresIn)
	assert.Equal(t, 5, resp.Interval)
	assert.Equal(t, "axonhub-test", gotUA)
	assert.Equal(t, "test-client", gotForm.Get("client_id"))
	assert.Equal(t, "read:user repo", gotForm.Get("scope"))
}

// Test Start() without scopes.
func TestDeviceFlowProvider_Start_NoScopes(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotForm, _ = url.ParseQuery(string(body))

		resp := DeviceFlowResponse{
			DeviceCode:      "device-code",
			UserCode:        "USER-CODE",
			VerificationURI: "https://example.com/verify",
			ExpiresIn:       900,
			Interval:        5,
		}
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "test-client",
		},
	})

	ctx := context.Background()
	_, err := provider.Start(ctx)
	require.NoError(t, err)
	assert.Empty(t, gotForm.Get("scope"))
}

// Test Start() error response.
func TestDeviceFlowProvider_Start_ErrorResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errResp := TokenError{
			Error:            "invalid_client",
			ErrorDescription: "client not found",
		}
		b, _ := json.Marshal(errResp)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "invalid-client",
		},
	})

	ctx := context.Background()
	_, err := provider.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device authorization request failed")
	assert.Contains(t, err.Error(), "400")
}

// Test Start() missing device_code.
func TestDeviceFlowProvider_Start_MissingDeviceCode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "test-client",
		},
	})

	ctx := context.Background()
	_, err := provider.Start(ctx)
	require.Error(t, err)
	assert.Equal(t, "device authorization response missing device_code", err.Error())
}

// Test Poll() validation errors.
func TestDeviceFlowProvider_Poll_Validation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name       string
		params     DeviceFlowProviderParams
		deviceCode string
		wantErr    string
	}{
		{
			name:       "nil http client",
			params:     DeviceFlowProviderParams{},
			deviceCode: "test-code",
			wantErr:    "http client is nil",
		},
		{
			name: "empty token URL",
			params: DeviceFlowProviderParams{
				HTTPClient: httpclient.NewHttpClient(),
			},
			deviceCode: "test-code",
			wantErr:    "token URL is empty",
		},
		{
			name: "empty device code",
			params: DeviceFlowProviderParams{
				HTTPClient: httpclient.NewHttpClient(),
				Config: DeviceFlowConfig{
					TokenURL: "https://example.com/token",
				},
			},
			deviceCode: "",
			wantErr:    "device_code is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewDeviceFlowProvider(tt.params)
			_, err := provider.Poll(ctx, tt.deviceCode)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

// Test Poll() success.
func TestDeviceFlowProvider_Poll_Success(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/token", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotForm, err = url.ParseQuery(string(body))
		require.NoError(t, err)

		resp := TokenResponse{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-456",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			Scope:        "read:user repo",
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "test-client",
		},
	})

	ctx := context.Background()
	creds, err := provider.Poll(ctx, "device-code-123")
	require.NoError(t, err)
	require.NotNil(t, creds)

	assert.Equal(t, "urn:ietf:params:oauth:grant-type:device_code", gotForm.Get("grant_type"))
	assert.Equal(t, "test-client", gotForm.Get("client_id"))
	assert.Equal(t, "device-code-123", gotForm.Get("device_code"))
	assert.Equal(t, "test-client", creds.ClientID)
	assert.Equal(t, "access-token-123", creds.AccessToken)
	assert.Equal(t, "refresh-token-456", creds.RefreshToken)
	assert.Equal(t, "Bearer", creds.TokenType)
	assert.Equal(t, []string{"read:user", "repo"}, creds.Scopes)
	assert.False(t, creds.ExpiresAt.IsZero())

	stored := provider.GetCredentials()
	require.NotNil(t, stored)
	assert.Equal(t, creds.AccessToken, stored.AccessToken)
}

// Test Poll() error responses (RFC 8628 device flow errors).
func TestDeviceFlowProvider_Poll_ErrorResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		errorCode string
		wantErr   string
	}{
		{
			name:      "authorization_pending",
			errorCode: "authorization_pending",
			wantErr:   "authorization_pending",
		},
		{
			name:      "slow_down",
			errorCode: "slow_down",
			wantErr:   "slow_down",
		},
		{
			name:      "expired_token",
			errorCode: "expired_token",
			wantErr:   "expired_token",
		},
		{
			name:      "access_denied",
			errorCode: "access_denied",
			wantErr:   "access_denied",
		},
		{
			name:      "custom error",
			errorCode: "custom_error",
			wantErr:   "token request failed: custom_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				errResp := TokenError{
					Error:            tt.errorCode,
					ErrorDescription: "detailed description",
				}
				b, _ := json.Marshal(errResp)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(b)
			}))
			defer server.Close()

			provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
				HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
				Config: DeviceFlowConfig{
					DeviceAuthURL: server.URL + "/device",
					TokenURL:      server.URL + "/token",
					ClientID:      "test-client",
				},
			})

			ctx := context.Background()
			_, err := provider.Poll(ctx, "device-code")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// Test GetToken() with no credentials.
func TestDeviceFlowProvider_GetToken_NoCredentials(t *testing.T) {
	t.Parallel()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
	})

	ctx := context.Background()
	_, err := provider.GetToken(ctx)
	require.Error(t, err)
	assert.Equal(t, "credentials is nil", err.Error())
}

// Test GetToken() with empty access token.
func TestDeviceFlowProvider_GetToken_EmptyAccessToken(t *testing.T) {
	t.Parallel()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
		Credentials: &OAuthCredentials{
			AccessToken: "",
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	})

	ctx := context.Background()
	_, err := provider.GetToken(ctx)
	require.Error(t, err)
	assert.Equal(t, "access token is empty", err.Error())
}

// Test GetToken() with valid non-expired token.
func TestDeviceFlowProvider_GetToken_ValidToken(t *testing.T) {
	t.Parallel()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
		Credentials: &OAuthCredentials{
			AccessToken:  "valid-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	})

	ctx := context.Background()
	token, err := provider.GetToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "valid-token", token)
}

// Test GetToken() with token exchanger.
func TestDeviceFlowProvider_GetToken_WithExchanger(t *testing.T) {
	t.Parallel()

	exchanger := &mockTokenExchanger{
		token:     "exchanged-token",
		expiresAt: time.Now().Add(time.Hour).Unix(),
	}

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient:     httpclient.NewHttpClient(),
		TokenExchanger: exchanger,
		Credentials: &OAuthCredentials{
			AccessToken:  "oauth-access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	})

	ctx := context.Background()
	token, err := provider.GetToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "exchanged-token", token)
}

// Test GetToken() with token exchanger error.
func TestDeviceFlowProvider_GetToken_ExchangerError(t *testing.T) {
	t.Parallel()

	exchanger := &mockTokenExchanger{
		err: errors.New("exchange failed"),
	}

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient:     httpclient.NewHttpClient(),
		TokenExchanger: exchanger,
		Credentials: &OAuthCredentials{
			AccessToken:  "oauth-access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	})

	ctx := context.Background()
	_, err := provider.GetToken(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exchange failed")
}

// Test GetToken() with expired token refresh.
func TestDeviceFlowProvider_GetToken_Refresh(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotForm, err = url.ParseQuery(string(body))
		require.NoError(t, err)

		resp := TokenResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	var refreshedCalled atomic.Int32

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "test-client",
		},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired-token",
			RefreshToken: "old-refresh-token",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		OnRefreshed: func(ctx context.Context, refreshed *OAuthCredentials) error {
			refreshedCalled.Add(1)
			return nil
		},
	})

	ctx := context.Background()
	token, err := provider.GetToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", token)

	assert.Equal(t, "refresh_token", gotForm.Get("grant_type"))
	assert.Equal(t, "test-client", gotForm.Get("client_id"))
	assert.Equal(t, "old-refresh-token", gotForm.Get("refresh_token"))
	assert.Equal(t, int32(1), refreshedCalled.Load())

	stored := provider.GetCredentials()
	require.NotNil(t, stored)
	assert.Equal(t, "new-access-token", stored.AccessToken)
	assert.Equal(t, "new-refresh-token", stored.RefreshToken)
}

// Test GetToken() refresh without refresh_token.
func TestDeviceFlowProvider_GetToken_Refresh_NoRefreshToken(t *testing.T) {
	t.Parallel()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
		Config: DeviceFlowConfig{
			TokenURL: "https://example.com/token",
		},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired-token",
			RefreshToken: "",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	ctx := context.Background()
	_, err := provider.GetToken(ctx)
	require.Error(t, err)
	assert.Equal(t, "refresh_token is empty", err.Error())
}

// Test GetToken() concurrent deduplication with singleflight.
func TestDeviceFlowProvider_GetToken_Singleflight(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = calls.Add(1)

		time.Sleep(50 * time.Millisecond)

		resp := TokenResponse{
			AccessToken:  "refreshed-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		}
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			TokenURL: server.URL + "/token",
			ClientID: "test-client",
		},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired-token",
			RefreshToken: "old-refresh-token",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	ctx := context.Background()

	var wg sync.WaitGroup

	tokens := make([]string, 10)
	errors := make([]error, 10)

	for i := range 10 {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			token, err := provider.GetToken(ctx)
			tokens[idx] = token
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		require.NoError(t, err, "goroutine %d failed", i)
	}

	for i := 1; i < 10; i++ {
		assert.Equal(t, tokens[0], tokens[i], "goroutine %d got different token", i)
	}

	assert.Equal(t, int32(1), calls.Load())
}

// Test UpdateCredentials and GetCredentials.
func TestDeviceFlowProvider_CredentialsManagement(t *testing.T) {
	t.Parallel()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
	})

	creds := provider.GetCredentials()
	assert.Nil(t, creds)

	original := &OAuthCredentials{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	provider.UpdateCredentials(original)

	stored := provider.GetCredentials()
	require.NotNil(t, stored)
	assert.Equal(t, "access-1", stored.AccessToken)
	assert.Equal(t, "refresh-1", stored.RefreshToken)
	assert.False(t, stored.ExpiresAt.IsZero())

	stored.AccessToken = "modified"
	stored2 := provider.GetCredentials()
	assert.Equal(t, "access-1", stored2.AccessToken)

	original.AccessToken = "original-modified"
	stored3 := provider.GetCredentials()
	assert.Equal(t, "access-1", stored3.AccessToken)

	provider.UpdateCredentials(nil)
	assert.Nil(t, provider.GetCredentials())
}

// Test OnRefreshed callback.
func TestDeviceFlowProvider_OnRefreshed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			AccessToken:  "new-token",
			RefreshToken: "new-refresh",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		}
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	var (
		receivedCreds  *OAuthCredentials
		callbackCalled atomic.Int32
	)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			TokenURL: server.URL + "/token",
			ClientID: "test-client",
		},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		OnRefreshed: func(ctx context.Context, refreshed *OAuthCredentials) error {
			callbackCalled.Add(1)

			receivedCreds = refreshed

			return nil
		},
	})

	ctx := context.Background()
	_, err := provider.GetToken(ctx)
	require.NoError(t, err)

	assert.Equal(t, int32(1), callbackCalled.Load())
	require.NotNil(t, receivedCreds)
	assert.Equal(t, "new-token", receivedCreds.AccessToken)
	assert.Equal(t, "new-refresh", receivedCreds.RefreshToken)
}

// Test OnRefreshed callback error (should not fail GetToken).
func TestDeviceFlowProvider_OnRefreshed_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			AccessToken:  "new-token",
			RefreshToken: "new-refresh",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		}
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			TokenURL: server.URL + "/token",
			ClientID: "test-client",
		},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		OnRefreshed: func(ctx context.Context, refreshed *OAuthCredentials) error {
			return errors.New("persistence failed")
		},
	})

	ctx := context.Background()
	token, err := provider.GetToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
}

// Test concurrent credential updates.
func TestDeviceFlowProvider_ConcurrentCredentials(t *testing.T) {
	t.Parallel()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
	})

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			provider.UpdateCredentials(&OAuthCredentials{
				AccessToken:  "token",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(time.Hour),
			})
		}(i)
	}

	for range 100 {
		wg.Go(func() {
			_ = provider.GetCredentials()
		})
	}

	wg.Wait()

	creds := provider.GetCredentials()
	require.NotNil(t, creds)
}

// Test internal refresh method.
func TestDeviceFlowProvider_refresh(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil credentials", func(t *testing.T) {
		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClient(),
		})
		_, err := provider.refresh(ctx, nil)
		require.Error(t, err)
		assert.Equal(t, "nil credentials", err.Error())
	})

	t.Run("empty refresh token", func(t *testing.T) {
		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClient(),
		})
		_, err := provider.refresh(ctx, &OAuthCredentials{})
		require.Error(t, err)
		assert.Equal(t, "refresh_token is empty", err.Error())
	})

	t.Run("empty token URL", func(t *testing.T) {
		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClient(),
		})
		_, err := provider.refresh(ctx, &OAuthCredentials{RefreshToken: "refresh"})
		require.Error(t, err)
		assert.Equal(t, "token URL is empty", err.Error())
	})

	t.Run("successful refresh", func(t *testing.T) {
		var gotForm url.Values

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			gotForm, _ = url.ParseQuery(string(body))

			resp := TokenResponse{
				AccessToken:  "new-access",
				RefreshToken: "new-refresh",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}
			b, _ := json.Marshal(resp)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
		}))
		defer server.Close()

		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
			Config: DeviceFlowConfig{
				TokenURL: server.URL + "/token",
				ClientID: "test-client",
			},
		})

		oldCreds := &OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		}

		newCreds, err := provider.refresh(ctx, oldCreds)
		require.NoError(t, err)

		assert.Equal(t, "refresh_token", gotForm.Get("grant_type"))
		assert.Equal(t, "test-client", gotForm.Get("client_id"))
		assert.Equal(t, "old-refresh", gotForm.Get("refresh_token"))
		assert.Equal(t, "new-access", newCreds.AccessToken)
		assert.Equal(t, "new-refresh", newCreds.RefreshToken)
	})

	t.Run("preserves refresh token if not returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := TokenResponse{
				AccessToken: "new-access",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			}
			b, _ := json.Marshal(resp)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
		}))
		defer server.Close()

		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
			Config: DeviceFlowConfig{
				TokenURL: server.URL + "/token",
				ClientID: "test-client",
			},
		})

		oldCreds := &OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		}

		newCreds, err := provider.refresh(ctx, oldCreds)
		require.NoError(t, err)

		assert.Equal(t, "new-access", newCreds.AccessToken)
		assert.Equal(t, "old-refresh", newCreds.RefreshToken)
	})
}

// Test refresh with UserAgent header.
func TestDeviceFlowProvider_refresh_UserAgent(t *testing.T) {
	t.Parallel()

	var gotUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")

		resp := TokenResponse{
			AccessToken: "new-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer server.Close()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			TokenURL:  server.URL + "/token",
			ClientID:  "test-client",
			UserAgent: "custom-agent/1.0",
		},
	})

	ctx := context.Background()
	_, err := provider.refresh(ctx, &OAuthCredentials{RefreshToken: "old-refresh"})
	require.NoError(t, err)
	assert.Equal(t, "custom-agent/1.0", gotUA)
}

// Test StartAutoRefresh and StopAutoRefresh idempotency.
func TestDeviceFlowProvider_AutoRefresh_StartStop_Idempotent(t *testing.T) {
	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
		Credentials: &OAuthCredentials{
			AccessToken:  "access",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	})

	provider.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 1 * time.Minute,
	})

	provider.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 1 * time.Minute,
	})

	time.Sleep(30 * time.Millisecond)

	provider.StopAutoRefresh()
	provider.StopAutoRefresh()
}

// Test network errors.
func TestDeviceFlowProvider_NetworkErrors(t *testing.T) {
	t.Parallel()

	t.Run("Start() network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
			Config: DeviceFlowConfig{
				DeviceAuthURL: server.URL + "/device",
				ClientID:      "test",
			},
		})

		_, err := provider.Start(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device authorization request failed")
	})

	t.Run("Poll() network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
			Config: DeviceFlowConfig{
				TokenURL: server.URL + "/token",
				ClientID: "test",
			},
		})

		_, err := provider.Poll(context.Background(), "device-code")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token request failed")
	})
}

// Test invalid JSON responses.
func TestDeviceFlowProvider_InvalidJSON(t *testing.T) {
	t.Parallel()

	t.Run("Start() invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
			Config: DeviceFlowConfig{
				DeviceAuthURL: server.URL + "/device",
				ClientID:      "test",
			},
		})

		_, err := provider.Start(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode device authorization response")
	})

	t.Run("Poll() invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
			HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
			Config: DeviceFlowConfig{
				TokenURL: server.URL + "/token",
				ClientID: "test",
			},
		})

		_, err := provider.Poll(context.Background(), "device-code")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode response")
	})
}

// Test scope parsing.
func TestDeviceFlowProvider_ScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		scope          string
		expectedScopes []string
	}{
		{
			name:           "single scope",
			scope:          "read:user",
			expectedScopes: []string{"read:user"},
		},
		{
			name:           "multiple scopes",
			scope:          "read:user repo write:repo",
			expectedScopes: []string{"read:user", "repo", "write:repo"},
		},
		{
			name:           "empty scope",
			scope:          "",
			expectedScopes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := TokenResponse{
					AccessToken: "token",
					TokenType:   "Bearer",
					ExpiresIn:   3600,
					Scope:       tt.scope,
				}
				b, _ := json.Marshal(resp)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(b)
			}))
			defer server.Close()

			provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
				HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
				Config: DeviceFlowConfig{
					TokenURL: server.URL + "/token",
					ClientID: "test-client",
				},
			})

			creds, err := provider.Poll(context.Background(), "device-code")
			require.NoError(t, err)

			if tt.expectedScopes == nil {
				assert.Empty(t, creds.Scopes)
			} else {
				assert.Equal(t, tt.expectedScopes, creds.Scopes)
			}
		})
	}
}

func TestDeviceFlowProvider_Poll_IDToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			IDToken:      "id-token-value",
		}
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer server.Close()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			TokenURL: server.URL + "/token",
			ClientID: "test-client",
		},
	})

	creds, err := provider.Poll(context.Background(), "device-code")
	require.NoError(t, err)
	require.NotNil(t, creds)

	assert.Equal(t, "id-token-value", creds.IDToken)
}

// Test complete device flow integration.
func TestDeviceFlowProvider_Integration_FullFlow(t *testing.T) {
	t.Parallel()

	var tokenRequestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			resp := DeviceFlowResponse{
				DeviceCode:      "device-code-123",
				UserCode:        "USER-123",
				VerificationURI: "https://example.com/verify",
				ExpiresIn:       900,
				Interval:        5,
			}
			b, _ := json.Marshal(resp)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)

		case "/token":
			count := tokenRequestCount.Add(1)

			if count <= 2 {
				errResp := TokenError{Error: "authorization_pending"}
				b, _ := json.Marshal(errResp)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(b)

				return
			}

			resp := TokenResponse{
				AccessToken:  "final-access-token",
				RefreshToken: "final-refresh-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}
			b, _ := json.Marshal(resp)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
		}
	}))
	defer server.Close()

	provider := NewDeviceFlowProvider(DeviceFlowProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		Config: DeviceFlowConfig{
			DeviceAuthURL: server.URL + "/device",
			TokenURL:      server.URL + "/token",
			ClientID:      "test-client",
			Scopes:        []string{"read:user"},
		},
	})

	ctx := context.Background()

	startResp, err := provider.Start(ctx)
	require.NoError(t, err)
	require.NotNil(t, startResp)

	var creds *OAuthCredentials
	for range 5 {
		creds, err = provider.Poll(ctx, startResp.DeviceCode)
		if err == nil {
			break
		}

		if err.Error() == "authorization_pending" {
			continue
		}

		t.Fatalf("unexpected error: %v", err)
	}

	require.NotNil(t, creds)
	assert.Equal(t, "final-access-token", creds.AccessToken)
	assert.Equal(t, "final-refresh-token", creds.RefreshToken)
}
