package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestVideoCreate_Text(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestVideoCreate_SDK")
	ctx := helper.CreateTestContext()

	videoModel := os.Getenv("TEST_VIDEO_MODEL")
	if strings.TrimSpace(videoModel) == "" {
		t.Skip("TEST_VIDEO_MODEL is required (e.g. sora-2 or a gateway-supported video model)")
	}

	seconds, err := getVideoSeconds()
	helper.AssertNoError(t, err, "Invalid TEST_VIDEO_SECONDS")

	size := strings.TrimSpace(os.Getenv("TEST_VIDEO_SIZE"))
	if size == "" {
		size = "1280x720"
	}

	video, err := helper.Client.Videos.New(ctx, openai.VideoNewParams{
		Prompt:  "A cat walking on the beach at sunset",
		Model:   openai.VideoModel(videoModel),
		Seconds: seconds,
		Size:    openai.VideoSize(size),
	}, helper.GetHeaderOptions()...)
	helper.AssertNoError(t, err, "Failed to create video")

	if video == nil || strings.TrimSpace(video.ID) == "" {
		t.Fatalf("Expected non-empty video id")
	}
	if strings.TrimSpace(string(video.Status)) == "" {
		t.Fatalf("Expected non-empty status")
	}
}

func TestVideoCreate_WithImage(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestVideoCreate_SDK_WithReferenceFile")
	ctx := helper.CreateTestContext()

	videoModel := os.Getenv("TEST_VIDEO_MODEL")
	if strings.TrimSpace(videoModel) == "" {
		t.Skip("TEST_VIDEO_MODEL is required (e.g. sora-2 or a gateway-supported video model)")
	}

	referenceImage := os.Getenv("TEST_VIDEO_REFERENCE_IMAGE")
	if strings.TrimSpace(referenceImage) == "" {
		t.Skip("TEST_VIDEO_REFERENCE_IMAGE is required (e.g. ref.png)")
	}

	seconds, err := getVideoSeconds()
	helper.AssertNoError(t, err, "Invalid TEST_VIDEO_SECONDS")

	size := strings.TrimSpace(os.Getenv("TEST_VIDEO_SIZE"))
	if size == "" {
		size = "1280x720"
	}

	video, err := helper.Client.Videos.New(ctx, openai.VideoNewParams{
		Prompt:         "A cat walking on the beach at sunset",
		InputReference: newNamedBytesReader(t, referenceImage, "image/png"),
		Model:          openai.VideoModel(videoModel),
		Seconds:        seconds,
		Size:           openai.VideoSize(size),
	}, helper.GetHeaderOptions()...)
	helper.AssertNoError(t, err, "Failed to create video with input_reference file")

	if video == nil || strings.TrimSpace(video.ID) == "" {
		t.Fatalf("Expected non-empty video id")
	}
}

func getVideoSeconds() (openai.VideoSeconds, error) {
	seconds := strings.TrimSpace(os.Getenv("TEST_VIDEO_SECONDS"))
	if seconds == "" {
		return openai.VideoSeconds4, nil
	}

	switch seconds {
	case "4":
		return openai.VideoSeconds4, nil
	case "8":
		return openai.VideoSeconds8, nil
	case "12":
		return openai.VideoSeconds12, nil
	default:
		return "4", fmt.Errorf("unsupported seconds %q (allowed: 4/8/12)", seconds)
	}
}

type namedBytesReader struct {
	*bytes.Reader
	filename    string
	contentType string
}

func newNamedBytesReader(t *testing.T, fp, contentType string) namedBytesReader {
	fileData, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("Failed to read reference image file: %v", err)
	}

	return namedBytesReader{
		Reader:      bytes.NewReader(fileData),
		filename:    filepath.Base(fp),
		contentType: contentType,
	}
}

func (b namedBytesReader) Filename() string    { return b.filename }
func (b namedBytesReader) ContentType() string { return b.contentType }
