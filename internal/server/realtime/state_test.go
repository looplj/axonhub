// 职责: 验证上游确认之前不会把实时会话标为可用。
// 关联文件:
//
//	internal/server/realtime/state.go

package realtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateRequiresConfigurationAcknowledgement(t *testing.T) {
	s := NewState()
	require.Equal(t, Created, s.Load())
	require.NoError(t, s.Connecting())
	require.False(t, s.Ready())
	require.NoError(t, s.ObserveServer("session.created"))
	require.False(t, s.Ready())
	require.NoError(t, s.ObserveServer("session.updated"))
	require.True(t, s.Ready())
	require.NoError(t, s.Update())
	require.False(t, s.Ready())
	require.NoError(t, s.ObserveServer("session.updated"))
	require.True(t, s.Ready())
	require.NoError(t, s.ObserveServer("response.created"))
	require.Equal(t, Active, s.Load())
	s.Close()
	require.Equal(t, Closed, s.Load())
	require.Error(t, s.Update())
}

func TestNativeStateAllowsInitialClientConfiguration(t *testing.T) {
	s := NewState()
	require.NoError(t, s.Connecting())
	require.NoError(t, s.ObserveServer("session.created"))
	s.AwaitClientConfiguration()
	require.False(t, s.Ready())
	require.NoError(t, s.Update())
	require.False(t, s.Ready())
	require.NoError(t, s.ObserveServer("session.updated"))
	require.True(t, s.Ready())
}

func TestNativeProviderConfirmedDefaultsAreUsable(t *testing.T) {
	s := NewState()
	require.NoError(t, s.Connecting())
	require.False(t, s.Ready())
	require.NoError(t, s.ObserveServer("session.created"))
	s.ConfirmDefaultConfiguration()
	require.True(t, s.Ready())
	require.NoError(t, s.Update())
	require.False(t, s.Ready())
	require.NoError(t, s.ObserveServer("session.updated"))
	require.True(t, s.Ready())
}

func TestStateReturnsPendingUpdateDuringGeneration(t *testing.T) {
	s := NewState()
	require.NoError(t, s.Connecting())
	require.NoError(t, s.ObserveServer("session.created"))
	s.ConfirmDefaultConfiguration()
	require.NoError(t, s.ObserveServer("response.created"))
	require.NoError(t, s.Update())
	err := s.Update()
	require.ErrorIs(t, err, ErrUpdatePending)
}
