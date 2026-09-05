package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoContentJSONRoundTrip(t *testing.T) {
	// Given
	want := []VideoContent{
		{Type: "text", Text: "Generate a sunset video"},
		{Type: "image_url", ImageURL: &VideoImageURL{URL: "data:image/png;base64,AAAA"}, Role: "first_frame"},
		{Type: "video_url", VideoURL: &VideoURL{URL: "data:video/mp4;base64,AAAA"}, Role: "reference_video"},
		{Type: "audio_url", AudioURL: &AudioURL{URL: "data:audio/wav;base64,AAAA"}, Role: "reference_audio"},
	}

	// When
	encoded, err := json.Marshal(want)
	require.NoError(t, err)
	var got []VideoContent
	require.NoError(t, json.Unmarshal(encoded, &got))

	// Then
	require.Equal(t, want, got)
}

func TestVideoContentJSONRejectsMalformedURLContainer(t *testing.T) {
	// Given
	raw := []byte(`[{"type":"video_url","video_url":"data:video/mp4;base64,AAAA"}]`)

	// When
	var got []VideoContent
	err := json.Unmarshal(raw, &got)

	// Then
	require.Error(t, err)
}

func TestVideoAPIFormatCapability(t *testing.T) {
	// Given
	want := map[string]struct{}{
		APIFormatOpenAIVideo.String():   {},
		APIFormatSeedanceVideo.String(): {},
		APIFormatZenmuxVideo.String():   {},
	}

	// When
	got := CapableAPIFormats(RequestTypeVideo)

	// Then
	require.Equal(t, want, got)
}
