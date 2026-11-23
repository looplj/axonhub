// 职责: 验证实时上游直连会在已校验地址间按序回退。
// 关联文件:
//   internal/server/realtime/dial.go

package realtime

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDialValidatedIPsTriesRemainingAddresses(t *testing.T) {
	first := net.ParseIP("8.8.8.8")
	second := net.ParseIP("1.1.1.1")
	var attempted []string
	wantErr := errors.New("first address is unavailable")
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	conn, err := dialValidatedIPs(context.Background(), "tcp", "443", []net.IPAddr{{IP: first}, {IP: second}}, func(_ context.Context, network, address string) (net.Conn, error) {
		require.Equal(t, "tcp", network)
		attempted = append(attempted, address)
		if len(attempted) == 1 {
			return nil, wantErr
		}
		return left, nil
	})

	require.NoError(t, err)
	require.Same(t, left, conn)
	require.Equal(t, []string{"8.8.8.8:443", "1.1.1.1:443"}, attempted)
}

func TestHandshakeErrorRetainsDeadlineCauseWithoutExposingIt(t *testing.T) {
	err := &HandshakeError{Status: 0, Cause: context.DeadlineExceeded}
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, err.Error(), "context deadline exceeded")
}

func TestHandshakeFailureReasonPreservesProviderKeywordsWithoutExposingCredentials(t *testing.T) {
	providerKey := "sk-provider-secret-value"
	body := `{"error":{"code":"invalid_api_key","message":"Invalid API key: ` + providerKey + `"}}`
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}

	reason := handshakeFailureReason(response, providerKey)
	require.Contains(t, reason, "invalid_api_key")
	require.Contains(t, reason, "Invalid API key")
	require.NotContains(t, reason, providerKey)
	require.LessOrEqual(t, len([]rune(reason)), maxHandshakeFailureReasonRunes)
}

func TestHandshakeFailureReasonTruncatesProviderText(t *testing.T) {
	response := &http.Response{Body: io.NopCloser(strings.NewReader(strings.Repeat("provider failure ", 100)))}

	reason := handshakeFailureReason(response, "")
	require.Contains(t, reason, "provider failure")
	require.LessOrEqual(t, len([]rune(reason)), maxHandshakeFailureReasonRunes)
}
