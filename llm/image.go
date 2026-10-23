package llm

import "encoding/json"

// ImageRequest is the unified image request structure (similar to EmbeddingRequest).
// Note: Common fields like Model are in the parent Request struct, not here.
type ImageRequest struct {
	ModelSpecified bool `json:"-"`
	// Prompt is the text prompt for image generation.
	Prompt string `json:"prompt,omitempty"`

	// Images contains the raw image data for edit/variation requests.
	Images          [][]byte         `json:"-"`
	ImageReferences []ImageReference `json:"images,omitempty"`

	// Mask is the mask image data for edit requests (optional).
	Mask          []byte          `json:"-"`
	MaskReference *ImageReference `json:"mask,omitempty"`

	// N is the number of images to generate.
	N *int64 `json:"n,omitempty"`

	// Size is the size of the generated images.
	Size string `json:"size,omitempty"`

	// Quality is the quality of the generated images.
	Quality string `json:"quality,omitempty"`

	// ResponseFormat is the format of the response (url or b64_json).
	ResponseFormat string `json:"response_format,omitempty"`

	// User is an optional unique identifier for the end-user.
	User string `json:"user,omitempty"`

	// Background is the background type (opaque or transparent) for GPT Image models.
	Background string `json:"background,omitempty"`

	// OutputFormat is the output image format (png, webp, jpeg) for GPT Image models.
	OutputFormat string `json:"output_format,omitempty"`

	// OutputCompression is the compression level (0-100) for GPT Image models.
	OutputCompression *int64 `json:"output_compression,omitempty"`

	// InputFidelity is the input fidelity for edit requests.
	InputFidelity string `json:"input_fidelity,omitempty"`

	// Moderation is the moderation level (low, auto) for generation requests.
	Moderation string `json:"moderation,omitempty"`

	// PartialImages is the number of partial images for GPT Image models.
	PartialImages *int64 `json:"partial_images,omitempty"`

	// Style is the style for DALL-E 3 (vivid or natural).
	Style string `json:"style,omitempty"`

	// SubjectReference contains provider-specific reference image descriptors.
	SubjectReference json.RawMessage `json:"subject_reference,omitempty"`

	// AspectRatio, Width and Height control provider-specific image dimensions.
	AspectRatio string `json:"aspect_ratio,omitempty"`
	Width       *int64 `json:"width,omitempty"`
	Height      *int64 `json:"height,omitempty"`

	// PromptOptimizer enables provider prompt optimization when supported.
	PromptOptimizer *bool `json:"prompt_optimizer,omitempty"`
}

type ImageReference struct {
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

// ImageResponse represents the unified image response model.
// Native image usage is retained here; parent Response.Usage is its accounting projection.
type ImageResponse struct {
	Created          int64           `json:"created"`
	Data             []ImageData     `json:"data"`
	Background       string          `json:"background,omitempty"`
	OutputFormat     string          `json:"output_format,omitempty"`
	Quality          string          `json:"quality,omitempty"`
	Size             string          `json:"size,omitempty"`
	SafetyIdentifier json.RawMessage `json:"safety_identifier,omitempty"`
	Usage            *ImageUsage     `json:"usage,omitempty"`
}

// ImageData represents a single image in the response.
type ImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	GenerationID  string `json:"generation_id,omitempty"`
}

type ImageUsage struct {
	InputTokens         int64                     `json:"input_tokens"`
	OutputTokens        int64                     `json:"output_tokens"`
	TotalTokens         int64                     `json:"total_tokens"`
	InputTokensDetails  *ImageInputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *ImageOutputTokensDetails `json:"output_tokens_details,omitempty"`
	Cost                *float64                  `json:"cost,omitempty"`
}

type ImageInputTokensDetails struct {
	ImageTokens         int64                     `json:"image_tokens"`
	TextTokens          int64                     `json:"text_tokens"`
	CachedTokens        *int64                    `json:"cached_tokens,omitempty"`
	CachedTokensDetails *ImageCachedTokensDetails `json:"cached_tokens_details,omitempty"`
}

type ImageCachedTokensDetails struct {
	ImageTokens int64 `json:"image_tokens"`
	TextTokens  int64 `json:"text_tokens"`
}

type ImageOutputTokensDetails struct {
	ImageTokens     *int64 `json:"image_tokens,omitempty"`
	TextTokens      *int64 `json:"text_tokens,omitempty"`
	ReasoningTokens *int64 `json:"reasoning_tokens,omitempty"`
}
