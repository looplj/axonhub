package llm

// VideoRequest is the unified video generation request model (async task).
// It is designed based on Seedance's content[] structure for extensibility.
type VideoRequest struct {
	// Model is the model ID.
	Model string `json:"model"`

	// Content is the input list (text prompt and image inputs).
	Content []VideoContent `json:"content"`

	// Duration is the video duration in seconds.
	Duration *int64 `json:"duration,omitempty"`

	// Ratio is the aspect ratio, e.g. "16:9", "9:16".
	Ratio string `json:"ratio,omitempty"`

	// Resolution is the resolution level, e.g. "480p", "720p", "1080p".
	Resolution string `json:"resolution,omitempty"`

	// Size is OpenAI-style resolution string, e.g. "1280x720", "720x1280".
	Size string `json:"size,omitempty"`

	// Frames is the total frames (Seedance supports frames vs duration).
	Frames *int64 `json:"frames,omitempty"`

	// Seed is the random seed.
	Seed *int64 `json:"seed,omitempty"`

	// GenerateAudio indicates whether to generate audio (Seedance 1.5 pro).
	GenerateAudio *bool `json:"generate_audio,omitempty"`

	// CameraFixed indicates whether to fix camera.
	CameraFixed *bool `json:"camera_fixed,omitempty"`

	// Watermark indicates whether to add watermark.
	Watermark *bool `json:"watermark,omitempty"`

	// Draft indicates preview mode (Seedance 1.5 pro).
	Draft *bool `json:"draft,omitempty"`

	// ServiceTier is the service tier, e.g. "default" or "flex".
	ServiceTier string `json:"service_tier,omitempty"`

	// ExecutionExpiresAfter is the task execution timeout in seconds.
	ExecutionExpiresAfter *int64 `json:"execution_expires_after,omitempty"`
}

type VideoContent struct {
	// Type is "text" or "image_url".
	Type string `json:"type"`

	// Text is the prompt (when Type="text").
	Text string `json:"text,omitempty"`

	// ImageURL is the image input (when Type="image_url").
	ImageURL *VideoImageURL `json:"image_url,omitempty"`

	// Role is the image role (Seedance): "first_frame", "last_frame", "reference_image".
	Role string `json:"role,omitempty"`
}

type VideoImageURL struct {
	URL string `json:"url"`
}

// VideoResponse is the unified video task response (create/get).
type VideoResponse struct {
	ID string `json:"id"`

	// Status is unified status: "queued", "running", "succeeded", "failed".
	Status string `json:"status"`

	VideoURL string `json:"video_url,omitempty"`

	// Progress is 0-100 (OpenAI-style).
	Progress *float64 `json:"progress,omitempty"`

	Model string `json:"model,omitempty"`
	// Prompt is a human-readable prompt for convenience.
	Prompt string `json:"prompt,omitempty"`

	Duration   *int64 `json:"duration,omitempty"`
	Size       string `json:"size,omitempty"`
	Ratio      string `json:"ratio,omitempty"`
	Resolution string `json:"resolution,omitempty"`

	FPS  *int64 `json:"fps,omitempty"`
	Seed *int64 `json:"seed,omitempty"`

	Error *VideoError `json:"error,omitempty"`

	CreatedAt   int64 `json:"created_at,omitempty"`
	CompletedAt int64 `json:"completed_at,omitempty"`
	ExpiresAt   int64 `json:"expires_at,omitempty"`
}

type VideoError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
