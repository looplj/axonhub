// 职责: 锁定 Codex usage 探测的状态码分类，确保只有 401 被当作凭据失效。
// 关联文件:
//
//	internal/server/biz/provider_quota/codex_checker.go
//	internal/server/biz/provider_quota/types.go
//
// 注意事项:
//
//	403、429、5xx、传输失败和解析失败必须保持普通探测错误，不能被扩成凭据失效。
package provider_quota

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestCodexQuotaChecker_CheckQuotaCredentialClassification pins which usage
// probe failures mean the stored credential is dead. Only HTTP 401 is a
// credential problem; 403, 429, 5xx, transport failures and parse failures stay
// ordinary check errors so their existing retry/keep semantics do not change.
func TestCodexQuotaChecker_CheckQuotaCredentialClassification(t *testing.T) {
	accessToken := buildCodexQuotaTestJWT(t, "acct_credential_classification")
	channelEntity := &ent.Channel{
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: accessToken},
		},
	}

	response := func(statusCode int, body string) roundTripFunc {
		return func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
	}

	tests := []struct {
		name             string
		transport        roundTripFunc
		wantInvalidCreds bool
	}{
		{
			name:             "401 marks credentials invalid",
			transport:        response(http.StatusUnauthorized, `{"error":"unauthorized"}`),
			wantInvalidCreds: true,
		},
		{
			name:      "403 stays an ordinary error",
			transport: response(http.StatusForbidden, `{"error":"forbidden"}`),
		},
		{
			name:      "429 stays an ordinary error",
			transport: response(http.StatusTooManyRequests, `{"error":"rate limited"}`),
		},
		{
			name:      "500 stays an ordinary error",
			transport: response(http.StatusInternalServerError, `{"error":"server error"}`),
		},
		{
			name: "network failure stays an ordinary error",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection reset by peer")
			},
		},
		{
			name:      "invalid JSON stays an ordinary error",
			transport: response(http.StatusOK, `<html>not json</html>`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCodexQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{Transport: tt.transport}))

			_, err := checker.CheckQuota(context.Background(), channelEntity)
			require.Error(t, err)
			require.Equal(t, tt.wantInvalidCreds, errors.Is(err, ErrInvalidCredentials), "unexpected classification: %v", err)
		})
	}
}
