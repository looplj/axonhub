package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageEditJSONSchemaContract(t *testing.T) {
	// Given the official JSON images/mask reference shape, independently of HTTP routing.
	body := `{"prompt":"synthetic","images":[{"image_url":"https://example.invalid/source.png"},{"image_url":"data:image/png;base64,c3ludGhldGlj"},{"file_id":"file-synthetic"}],"mask":{"file_id":"file-synthetic-mask"},"n":1,"size":"1024x1024","quality":"low","input_fidelity":"high","output_format":"webp","output_compression":0,"moderation":"low","background":"transparent","user":"synthetic-user"}`
	var request ImageRequest
	require.NoError(t, json.Unmarshal([]byte(body), &request))
	// When the shared image contract is serialized for persistence or a subsequent transformer.
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	// Then URL, data URL, file ID, mask and nonstreaming optional fields survive unchanged.
	require.JSONEq(t, body, string(encoded))
}
