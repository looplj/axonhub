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
	// PostgreSQL jsonb adds a space after the colon when scanning the marker
	// back into JSONRawMessage. Marker detection must compare JSON semantics,
	// rather than the exact byte representation written to the database.
	spacedMarker := objects.JSONRawMessage(` {"_ext": 1} `)
	require.True(t, isExternalResponseBodyMarker(spacedMarker))
	require.Equal(t, xjson.EmptyJSONRawMessage, sanitizeLoadedResponseBody(spacedMarker))
	require.Equal(t, xjson.EmptyJSONRawMessage, sanitizeLoadedResponseBody(nil))
	require.JSONEq(t, `{"ok":true}`, string(sanitizeLoadedResponseBody(objects.JSONRawMessage(`{"ok":true}`))))

	require.Empty(t, sanitizeLoadedResponseChunks(ExternalResponseChunksMarker))
	require.True(t, isExternalResponseChunksMarker([]objects.JSONRawMessage{spacedMarker}))
	require.Empty(t, sanitizeLoadedResponseChunks([]objects.JSONRawMessage{spacedMarker}))
	require.Empty(t, sanitizeLoadedResponseChunks(nil))

	realChunks := []objects.JSONRawMessage{objects.JSONRawMessage(`{"delta":"x"}`)}
	require.Equal(t, realChunks, sanitizeLoadedResponseChunks(realChunks))
}
