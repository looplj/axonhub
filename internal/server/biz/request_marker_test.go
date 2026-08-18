package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xjson"
)

func TestSanitizeLoadedExternalMarkers(t *testing.T) {
	t.Parallel()

	require.Equal(t, xjson.EmptyJSONRawMessage, sanitizeLoadedResponseBody(ExternalResponseBodyMarker))
	require.Equal(t, xjson.EmptyJSONRawMessage, sanitizeLoadedResponseBody(nil))
	require.JSONEq(t, `{"ok":true}`, string(sanitizeLoadedResponseBody(objects.JSONRawMessage(`{"ok":true}`))))

	require.Empty(t, sanitizeLoadedResponseChunks(ExternalResponseChunksMarker))
	require.Empty(t, sanitizeLoadedResponseChunks(nil))

	realChunks := []objects.JSONRawMessage{objects.JSONRawMessage(`{"delta":"x"}`)}
	require.Equal(t, realChunks, sanitizeLoadedResponseChunks(realChunks))
}
