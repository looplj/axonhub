package llm

import "encoding/json"

// SpeechStreamResponseID is the synthetic response ID used for aggregated
// streaming TTS metadata.
const SpeechStreamResponseID = "audio.speech.stream"

// SpeechRequest is the unified text-to-speech (TTS) request structure.
// It maps to OpenAI's POST /v1/audio/speech API.
// Note: Common fields like Model are in the parent Request struct, not here.
type SpeechRequest struct {
	// Input is the text to generate audio for.
	Input string `json:"input"`

	// Voice is the voice to use when generating the audio (e.g. alloy, echo, nova).
	Voice string `json:"voice"`

	// ResponseFormat is the audio format (mp3, opus, aac, flac, wav, pcm). Defaults to mp3.
	ResponseFormat string `json:"response_format,omitempty"`

	// Speed is the speed of the generated audio, from 0.25 to 4.0. Defaults to 1.0.
	Speed *float64 `json:"speed,omitempty"`

	// Instructions controls the voice tone for supported models (e.g. gpt-4o-mini-tts).
	Instructions string `json:"instructions,omitempty"`

	// StreamFormat selects streaming output for /audio/speech. OpenAI supports "sse" and "audio"; audio is the default.
	StreamFormat string `json:"stream_format,omitempty"`
}

// SpeechStreamEvent represents a single SSE event emitted by the OpenAI streaming TTS API.
// The unified gateway carries these events transparently for stream_format="sse".
type SpeechStreamEvent struct {
	// Type is the SSE event type, e.g. "speech.audio.delta" or "speech.audio.done".
	Type string `json:"type"`

	// AudioBase64 holds the base64-encoded audio chunk for delta events.
	AudioBase64 string `json:"audio,omitempty"`

	// Usage is populated on the terminal speech.audio.done event.
	Usage *Usage `json:"usage,omitempty"`
}

// SpeechAudioChunk represents one binary audio chunk emitted by streaming TTS
// when the provider returns raw chunked audio instead of SSE.
type SpeechAudioChunk struct {
	// Audio is the raw audio bytes for this chunk.
	Audio []byte `json:"-"`

	// ContentType is the MIME type returned by the provider for the stream.
	ContentType string `json:"-"`
}

// SpeechResponse represents the unified TTS response.
// The audio bytes are binary and are not JSON serialized; they are streamed back to the client as-is.
type SpeechResponse struct {
	// Audio is the raw generated audio bytes.
	Audio []byte `json:"-"`

	// ContentType is the MIME type of the audio (e.g. audio/mpeg, audio/wav).
	ContentType string `json:"-"`
}

// TranscriptionRequest is the unified speech-to-text (STT) transcription request structure.
// It maps to OpenAI's POST /v1/audio/transcriptions API (multipart/form-data upload).
// Note: Common fields like Model are in the parent Request struct, not here.
type TranscriptionRequest struct {
	// File is the raw audio file bytes to transcribe.
	File []byte `json:"-"`

	// FileName is the original file name, used for the multipart upload to the provider.
	FileName string `json:"-"`

	// Language is the language of the input audio in ISO-639-1 format (optional).
	Language string `json:"language,omitempty"`

	// Prompt is an optional text to guide the model's style or continue a previous segment.
	Prompt string `json:"prompt,omitempty"`

	// ResponseFormat is the transcript format (json, text, srt, verbose_json, vtt). Defaults to json.
	ResponseFormat string `json:"response_format,omitempty"`

	// Temperature is the sampling temperature, between 0 and 1 (optional).
	Temperature *float64 `json:"temperature,omitempty"`

	// Extra holds additional multipart form fields not modeled above (e.g.
	// timestamp_granularities[], include[], chunking_strategy), forwarded to the
	// provider as-is to keep OpenAI compatibility. Values keep multiplicity for
	// repeated fields.
	Extra map[string][]string `json:"extra,omitempty"`
}

// TranscriptionStreamEvent represents a single SSE event emitted by the OpenAI streaming
// transcription/translation API. Carried transparently by the gateway when stream=true.
type TranscriptionStreamEvent struct {
	// Type is the SSE event type, e.g. "transcript.text.delta" or "transcript.text.done".
	Type string `json:"type"`

	// Delta is the incremental transcript fragment for delta events.
	Delta string `json:"delta,omitempty"`

	// Text is the full transcript on the terminal transcript.text.done event.
	Text string `json:"text,omitempty"`

	// Logprobs is preserved when present (verbose_json + include["logprobs"]).
	Logprobs json.RawMessage `json:"logprobs,omitempty"`

	// Usage is populated on the terminal transcript.text.done event.
	Usage *Usage `json:"usage,omitempty"`
}

// TranslationRequest is the unified speech-to-text (STT) translation request structure.
// It maps to OpenAI's POST /v1/audio/translations API (multipart/form-data upload).
// It is structurally identical to TranscriptionRequest but without a Language field,
// since translation always targets English.
type TranslationRequest struct {
	// File is the raw audio file bytes to translate.
	File []byte `json:"-"`

	// FileName is the original file name, used for the multipart upload to the provider.
	FileName string `json:"-"`

	// Prompt is an optional text to guide the model's style or continue a previous segment.
	Prompt string `json:"prompt,omitempty"`

	// ResponseFormat is the transcript format (json, text, srt, verbose_json, vtt). Defaults to json.
	ResponseFormat string `json:"response_format,omitempty"`

	// Temperature is the sampling temperature, between 0 and 1 (optional).
	Temperature *float64 `json:"temperature,omitempty"`

	// Extra holds additional multipart form fields not modeled above, forwarded to
	// the provider as-is to keep OpenAI compatibility.
	Extra map[string][]string `json:"extra,omitempty"`
}

// TranscriptionResponse represents the unified STT response, shared by transcription and translation.
type TranscriptionResponse struct {
	// Text is the transcribed/translated text (present for json/verbose_json formats).
	Text string `json:"text,omitempty"`

	// Language is the detected language (present for verbose_json format).
	Language string `json:"language,omitempty"`

	// Duration is the audio duration in seconds (present for verbose_json format).
	Duration *float64 `json:"duration,omitempty"`

	// Raw holds the raw provider response body. For JSON formats (json, verbose_json) it
	// preserves extra fields (segments, words, task) for lossless passthrough; for non-JSON
	// formats (text, srt, vtt) it is the raw text body.
	Raw []byte `json:"-"`

	// RawContentType is the MIME type of the raw response.
	RawContentType string `json:"-"`
}
