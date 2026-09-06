package video_storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenVideoStreamRejectsPrivateAndLocalAddresses(t *testing.T) {
	for _, videoURL := range []string{
		"http://127.0.0.1/video.mp4",
		"http://10.0.0.1/video.mp4",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/video.mp4",
		"http://100.64.0.1/video.mp4",
	} {
		t.Run(videoURL, func(t *testing.T) {
			_, _, err := openVideoStream(context.Background(), videoURL)

			require.Error(t, err)
			require.True(t, errors.Is(err, ErrUnsafeVideoURL), err)
		})
	}
}

func TestOpenVideoStreamRejectsNonHTTPURL(t *testing.T) {
	_, _, err := openVideoStream(context.Background(), "file:///etc/passwd")

	require.ErrorIs(t, err, ErrUnsafeVideoURL)
}
