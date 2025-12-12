package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"github.com/openai/openai-go/v3/shared/constant"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestResponsesImageGeneration(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	ctx := helper.CreateTestContext()

	imageModel := os.Getenv("TEST_IMAGE_MODEL")
	if imageModel == "" {
		t.Skip("TEST_IMAGE_MODEL is required (set to an image-capable model, e.g. gpt-image-1)")
	}

	prompt := "Generate a simple PNG image: a cute dog on the green land"

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		Tools: []responses.ToolUnionParam{
			{
				OfImageGeneration: &responses.ToolImageGenerationParam{
					Model:        imageModel,
					Type:         constant.ImageGeneration("image_generation"),
					OutputFormat: "png",
					Size:         "1024x1024",
					Quality:      "low",
				},
			},
		},
	}

	resp, err := helper.Client.Responses.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to generate image with Responses API")
	if resp == nil {
		t.Fatal("Response is nil")
	}

	var (
		foundImage bool
		imgBytes   []byte
	)

	for _, item := range resp.Output {
		if item.Type != "image_generation_call" {
			continue
		}

		imgCall := item.AsImageGenerationCall()
		result := imgCall.Result
		if result == "" {
			continue
		}

		// Result is expected to be base64. Some providers may return data URLs.
		b64 := result
		if strings.HasPrefix(result, "data:image/") {
			if comma := strings.Index(result, ","); comma >= 0 && comma+1 < len(result) {
				b64 = result[comma+1:]
			}
		}

		decoded, decErr := base64.StdEncoding.DecodeString(b64)
		if decErr != nil || len(decoded) == 0 {
			continue
		}

		imgBytes = decoded
		foundImage = true
		break
	}

	if !foundImage {
		t.Fatalf("Expected at least one output item of type image_generation_call with base64 image data")
	}

	// Basic sanity check that we got non-trivial binary.
	if len(imgBytes) < 32 {
		t.Fatalf("Expected image bytes to be non-trivial, got %d bytes", len(imgBytes))
	}

	// Save image to tmp directory
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(os.TempDir(), fmt.Sprintf("generated_image_%s.png", timestamp))

	err = os.WriteFile(filename, imgBytes, 0644)
	if err != nil {
		t.Fatalf("Failed to save image to %s: %v", filename, err)
	}

	t.Logf("Image saved to: %s", filename)
}

func TestResponsesImageGenerationStreaming(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	ctx := helper.CreateTestContext()

	imageModel := os.Getenv("TEST_IMAGE_MODEL")
	if imageModel == "" {
		t.Skip("TEST_IMAGE_MODEL is required (set to an image-capable model, e.g. gpt-image-1)")
	}

	prompt := "Generate a simple PNG image: a cute dog on the green land"

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		Tools: []responses.ToolUnionParam{
			{
				OfImageGeneration: &responses.ToolImageGenerationParam{
					Model:        imageModel,
					Type:         constant.ImageGeneration("image_generation"),
					OutputFormat: "png",
					Size:         "1024x1024",
					Quality:      "low",
				},
			},
		},
	}

	stream := helper.Client.Responses.NewStreaming(ctx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start Responses image streaming")

	var (
		events       int
		foundImage   bool
		decodedBytes []byte
	)

	for stream.Next() {
		event := stream.Current()
		events++

		if event.Type == "response.image_generation_call.partial_image" && event.PartialImageB64 != "" {
			b, err := base64.StdEncoding.DecodeString(event.PartialImageB64)
			if err == nil && len(b) > 0 {
				decodedBytes = append(decodedBytes, b...)
				foundImage = true
			}
		}

		if event.Type == "response.completed" {
			for _, item := range event.Response.Output {
				if item.Type != "image_generation_call" {
					continue
				}

				imgCall := item.AsImageGenerationCall()
				result := imgCall.Result
				if result == "" {
					continue
				}

				b64 := result
				if strings.HasPrefix(result, "data:image/") {
					if comma := strings.Index(result, ","); comma >= 0 && comma+1 < len(result) {
						b64 = result[comma+1:]
					}
				}

				b, err := base64.StdEncoding.DecodeString(b64)
				if err == nil && len(b) > 0 {
					decodedBytes = b
					foundImage = true
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		helper.AssertNoError(t, err, "Stream error occurred")
	}

	if events == 0 {
		t.Fatalf("Expected at least one streaming event")
	}

	if !foundImage {
		t.Fatalf("Expected image data from streaming (partial_image or response.completed output)")
	}

	if len(decodedBytes) < 32 {
		t.Fatalf("Expected image bytes to be non-trivial, got %d bytes", len(decodedBytes))
	}

	// Save image to tmp directory
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(os.TempDir(), fmt.Sprintf("generated_image_%s.png", timestamp))

	err := os.WriteFile(filename, decodedBytes, 0644)
	if err != nil {
		t.Fatalf("Failed to save image to %s: %v", filename, err)
	}

	t.Logf("Streaming mage saved to: %s", filename)
}
