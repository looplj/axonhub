package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

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
