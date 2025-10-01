package llm

// ImageGeneration is a permissive structure to carry image generation tool
// parameters. It mirrors the OpenRouter/OpenAI Responses API fields we care
// about, but is intentionally loose to allow forward-compatibility.
type ImageGeneration struct {
	// One of opaque, transparent.
	Background     string         `json:"background,omitempty"`
	InputFidelity  string         `json:"input_fidelity,omitempty"`
	InputImageMask map[string]any `json:"input_image_mask,omitempty"`
	// One of low, auto.
	Moderation string `json:"moderation,omitempty"`
	// The compression level (0-100%) for the generated images. Default: 100.
	OutputCompression *int64 `json:"output_compression,omitempty"`
	// One of png, webp, or jpeg. Default: png.
	OutputFormat string `json:"output_format,omitempty"`
	// The number of images to generate. Default: 1.
	PartialImages *int64 `json:"partial_images,omitempty"`
	// The quality of the image that will be generated.
	// auto (default value) will automatically select the best quality for the given model.
	// high, medium and low are supported for gpt-image-1.
	// hd and standard are supported for dall-e-3.
	// standard is the only option for dall-e-2.
	Quality string `json:"quality,omitempty"`
	// One of 256x256, 512x512, or 1024x1024. Default: 1024x1024.
	Size string `json:"size,omitempty"`

	// Whether to add a watermark to the generated image. Default: false.
	// For seedream model only.
	Watermark bool `json:"watermark,omitempty"`
}
