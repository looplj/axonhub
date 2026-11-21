// 职责: 验证实时上游直连会在已校验地址间按序回退。
// 关联文件:
//   internal/server/realtime/dial.go

package realtime

import (
	"context"
	"errors"
	"net"
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
