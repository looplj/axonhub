package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "ping.db")) + "?_fk=1"
	require.NoError(t, Ping(t.Context(), "sqlite3", dsn))

	require.ErrorContains(t, Ping(t.Context(), "oracle", dsn), "invalid dialect")
	require.Error(t, Ping(t.Context(), "sqlite3", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "missing", "dir", "x.db"))))
}
