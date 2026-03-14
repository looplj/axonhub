package tools

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePath(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "workspace")

	t.Run("keeps spaces in relative path", func(t *testing.T) {
		got, err := validatePath("Claw 001/no01/IDENTITY.md", workspace, false)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(workspace, "Claw 001/no01/IDENTITY.md"), got)
	})

	t.Run("trims matching quotes around path", func(t *testing.T) {
		got, err := validatePath(`"/Users/September_1/Software/AxonClaw/Claw 001/no01/IDENTITY.md"`, workspace, false)
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean("/Users/September_1/Software/AxonClaw/Claw 001/no01/IDENTITY.md"), got)
	})

	t.Run("rejects quoted path outside workspace when restricted", func(t *testing.T) {
		_, err := validatePath(`"/tmp/Claw 001/IDENTITY.md"`, workspace, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside the workspace")
	})
}
