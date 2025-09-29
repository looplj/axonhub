package llm

// ImageGeneration is a permissive structure to carry image generation tool
// parameters. It mirrors the OpenRouter/OpenAI Responses API fields we care
// about, but is intentionally loose to allow forward-compatibility.
type ImageGeneration struct {
	Model             string         `json:"model,omitempty"`
	Background        string         `json:"background,omitempty"`
	InputFidelity     string         `json:"input_fidelity,omitempty"`
	InputImageMask    map[string]any `json:"input_image_mask,omitempty"`
	Moderation        string         `json:"moderation,omitempty"`
	OutputCompression *int64         `json:"output_compression,omitempty"`
	// One of png, webp, or jpeg. Default: png.
	OutputFormat  string `json:"output_format,omitempty"`
	PartialImages *int64 `json:"partial_images,omitempty"`
	Quality       string `json:"quality,omitempty"`
	Size          string `json:"size,omitempty"`
}
