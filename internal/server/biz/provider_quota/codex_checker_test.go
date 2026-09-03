package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCodexQuotaChecker_UsesMinimalUsageHeaders(t *testing.T) {
	accessToken := buildCodexQuotaTestJWT(t, "acct_test")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/backend-api/wham/usage", req.URL.Path)
			require.Equal(t, "axonhub/1.0", req.Header.Get("User-Agent"))
			require.Empty(t, req.Header.Get("Originator"))
			require.Empty(t, req.Header.Get("Chatgpt-Account-Id"))
			require.Equal(t, "Bearer "+accessToken, req.Header.Get("Authorization"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plan_type":"plus","rate_limit":{"allowed":true}}`)),
			}, nil
		}),
	})

	checker := NewCodexQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: accessToken},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func buildCodexQuotaTestJWT(t *testing.T, accountID string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
		},
	})

	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	return signed
}

func TestCodexQuotaChecker_ListResets_ReturnsAvailableResets(t *testing.T) {
	accessToken := buildCodexQuotaTestJWT(t, "acct_reset")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "GET", req.Method)
			require.Equal(t, "/backend-api/wham/rate-limit-reset-credits", req.URL.Path)
			require.Equal(t, "Bearer "+accessToken, req.Header.Get("Authorization"))
			require.Equal(t, "acct_reset", req.Header.Get("Chatgpt-Account-Id"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"credits": [
						{"id": "cred_1", "status": "available", "reset_type": "codex_rate_limits", "granted_at": "2026-09-01T00:00:00Z", "expires_at": "2026-09-08T00:00:00Z"},
						{"id": "cred_2", "status": "redeemed"}
					],
					"available_count": 1
				}`)),
			}, nil
		}),
	})

	checker := NewCodexQuotaChecker(httpClient)
	resets, err := checker.ListResets(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: accessToken},
		},
	})

	require.NoError(t, err)
	require.True(t, resets.Supported)
	require.Len(t, resets.Resets, 1)
	require.Equal(t, "cred_1", resets.Resets[0].ID)
	require.Equal(t, "available", resets.Resets[0].Status)
	require.Equal(t, "codex_rate_limits", resets.Resets[0].Type)
	require.Equal(t, "2026-09-01T00:00:00Z", resets.Resets[0].GrantedAt.Format(time.RFC3339))
	require.Equal(t, "2026-09-08T00:00:00Z", resets.Resets[0].ExpiresAt.Format(time.RFC3339))
}

func TestCodexQuotaChecker_ListResets_ReturnsEmptyAvailability(t *testing.T) {
	accessToken := buildCodexQuotaTestJWT(t, "acct_reset")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"credits": [
						{"id": "cred_1", "status": "redeemed"}
					],
					"available_count": 0
				}`)),
			}, nil
		}),
	})

	checker := NewCodexQuotaChecker(httpClient)
	resets, err := checker.ListResets(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: accessToken},
		},
	})

	require.NoError(t, err)
	require.True(t, resets.Supported)
	require.Empty(t, resets.Resets)
}

func TestCodexQuotaChecker_Reset_ConsumesFirstAvailableReset(t *testing.T) {
	accessToken := buildCodexQuotaTestJWT(t, "acct_reset")
	requestCount := 0

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "/backend-api/wham/rate-limit-reset-credits", req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"credits": [
							{"id": "cred_1", "status": "redeemed"},
							{"id": "cred_2", "status": "available", "reset_type": "codex_rate_limits"}
						],
						"available_count": 1
					}`)),
				}, nil
			case 2:
				require.Equal(t, "POST", req.Method)
				require.Equal(t, "/backend-api/wham/rate-limit-reset-credits/consume", req.URL.Path)
				require.Equal(t, "acct_reset", req.Header.Get("Chatgpt-Account-Id"))

				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), `"credit_id":"cred_2"`)
				require.Contains(t, string(body), `"redeem_request_id":"`)

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"code": "reset",
						"windows_reset": 1,
						"credit": {"id": "cred_2", "status": "redeemed", "redeemed_at": "2026-06-13T13:12:31Z"}
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request: %d", requestCount)
				return nil, nil
			}
		}),
	})

	checker := NewCodexQuotaChecker(httpClient)
	err := checker.Reset(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: accessToken},
		},
	})

	require.NoError(t, err)
	require.Equal(t, 2, requestCount)
}

func TestCodexQuotaChecker_Reset_ReturnsErrorWhenNoAvailableReset(t *testing.T) {
	accessToken := buildCodexQuotaTestJWT(t, "acct_reset")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"credits": [{"id": "cred_1", "status": "redeemed"}],
					"available_count": 0
				}`)),
			}, nil
		}),
	})

	checker := NewCodexQuotaChecker(httpClient)
	err := checker.Reset(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: accessToken},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "no available codex reset credit")
}
