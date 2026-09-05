package zenmux

import "encoding/json"

type nativeContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Role     string          `json:"role,omitempty"`
	ImageURL *nativeMediaURL `json:"image_url,omitempty"`
	VideoURL *nativeMediaURL `json:"video_url,omitempty"`
	AudioURL *nativeMediaURL `json:"audio_url,omitempty"`
}

type nativeMediaURL struct {
	URL string `json:"url"`
}

type nativeCreateRequest struct {
	Model           string            `json:"model"`
	Content         []nativeContent   `json:"content"`
	Resolution      string            `json:"resolution,omitempty"`
	Ratio           string            `json:"ratio,omitempty"`
	Duration        *int64            `json:"duration,omitempty"`
	Seed            *int64            `json:"seed,omitempty"`
	GenerateAudio   *bool             `json:"generate_audio,omitempty"`
	Frames          *int64            `json:"frames,omitempty"`
	CameraFixed     *bool             `json:"camera_fixed,omitempty"`
	Watermark       *bool             `json:"watermark,omitempty"`
	Draft           *bool             `json:"draft,omitempty"`
	CallbackURL     string            `json:"callback_url,omitempty"`
	ReturnLastFrame *bool             `json:"return_last_frame,omitempty"`
	Tools           []json.RawMessage `json:"tools,omitempty"`
}

type nativeVideoResponse struct {
	ID            string              `json:"id"`
	Status        string              `json:"status"`
	Model         string              `json:"model,omitempty"`
	Content       json.RawMessage     `json:"content,omitempty"`
	ParsedContent *nativeVideoContent `json:"-"`
	Error         *struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
}

type nativeVideoContent struct {
	VideoURL     string `json:"video_url,omitempty"`
	LastFrameURL string `json:"last_frame_url,omitempty"`
}
