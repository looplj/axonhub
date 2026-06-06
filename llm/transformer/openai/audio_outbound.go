package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// SpeechRequestBody represents the JSON request body for the OpenAI /audio/speech API.
type SpeechRequestBody struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
	Instructions   string   `json:"instructions,omitempty"`
	// StreamFormat selects streaming output from /audio/speech. OpenAI supports "sse" and "audio"; audio is the default.
	StreamFormat string `json:"stream_format,omitempty"`
}

// buildSpeechRequest builds the HTTP request for the OpenAI text-to-speech API (/audio/speech).
// The request body is JSON; the response is binary audio.
func (t *OutboundTransformer) buildSpeechRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Speech == nil {
		return nil, fmt.Errorf("%w: speech request is nil in llm.Request", transformer.ErrInvalidRequest)
	}

	if llmReq.Speech.Input == "" {
		return nil, fmt.Errorf("%w: input is required for speech", transformer.ErrInvalidRequest)
	}

	if llmReq.Speech.Voice == "" {
		return nil, fmt.Errorf("%w: voice is required for speech", transformer.ErrInvalidRequest)
	}

	body, err := json.Marshal(SpeechRequestBody{
		Model:          llmReq.Model,
		Input:          llmReq.Speech.Input,
		Voice:          llmReq.Speech.Voice,
		ResponseFormat: llmReq.Speech.ResponseFormat,
		Speed:          llmReq.Speech.Speed,
		Instructions:   llmReq.Speech.Instructions,
		StreamFormat:   llmReq.Speech.StreamFormat,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal speech request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	if llmReq.Speech.StreamFormat == "sse" {
		// Streaming TTS returns text/event-stream.
		headers.Set("Accept", "text/event-stream")
	} else {
		// Non-streaming audio is returned as a binary stream; accept anything.
		headers.Set("Accept", "*/*")
	}

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         t.buildAudioURL("/audio/speech"),
		Headers:     headers,
		Body:        body,
		ContentType: "application/json",
		Auth: &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: t.config.APIKeyProvider.Get(ctx),
		},
		RequestType: string(llm.RequestTypeSpeech),
		APIFormat:   string(llm.APIFormatOpenAISpeech),
	}, nil
}

// buildTranscriptionRequest builds the multipart HTTP request for the OpenAI /audio/transcriptions API.
func (t *OutboundTransformer) buildTranscriptionRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Transcription == nil {
		return nil, fmt.Errorf("%w: transcription request is nil in llm.Request", transformer.ErrInvalidRequest)
	}

	tr := llmReq.Transcription
	if len(tr.File) == 0 {
		return nil, fmt.Errorf("%w: file is required for transcription", transformer.ErrInvalidRequest)
	}

	fields := map[string][]string{"model": {llmReq.Model}}
	if tr.Language != "" {
		fields["language"] = []string{tr.Language}
	}

	if tr.Prompt != "" {
		fields["prompt"] = []string{tr.Prompt}
	}

	if tr.ResponseFormat != "" {
		fields["response_format"] = []string{tr.ResponseFormat}
	}

	if tr.Temperature != nil {
		fields["temperature"] = []string{strconv.FormatFloat(*tr.Temperature, 'f', -1, 64)}
	}

	// Forward unmodeled fields (e.g. timestamp_granularities[], include[]) as-is.
	for name, values := range tr.Extra {
		fields[name] = values
	}

	stream := llmReq.Stream != nil && *llmReq.Stream
	if stream {
		fields["stream"] = []string{"true"}
	}

	return t.buildAudioMultipartRequest(ctx, "/audio/transcriptions", llm.RequestTypeTranscription, llm.APIFormatOpenAITranscription, tr.File, tr.FileName, fields, stream)
}

// buildTranslationRequest builds the multipart HTTP request for the OpenAI /audio/translations API.
func (t *OutboundTransformer) buildTranslationRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Translation == nil {
		return nil, fmt.Errorf("%w: translation request is nil in llm.Request", transformer.ErrInvalidRequest)
	}

	tr := llmReq.Translation
	if len(tr.File) == 0 {
		return nil, fmt.Errorf("%w: file is required for translation", transformer.ErrInvalidRequest)
	}

	fields := map[string][]string{"model": {llmReq.Model}}
	if tr.Prompt != "" {
		fields["prompt"] = []string{tr.Prompt}
	}

	if tr.ResponseFormat != "" {
		fields["response_format"] = []string{tr.ResponseFormat}
	}

	if tr.Temperature != nil {
		fields["temperature"] = []string{strconv.FormatFloat(*tr.Temperature, 'f', -1, 64)}
	}

	// Forward unmodeled fields as-is.
	for name, values := range tr.Extra {
		fields[name] = values
	}

	stream := llmReq.Stream != nil && *llmReq.Stream
	if stream {
		fields["stream"] = []string{"true"}
	}

	return t.buildAudioMultipartRequest(ctx, "/audio/translations", llm.RequestTypeTranslation, llm.APIFormatOpenAITranslation, tr.File, tr.FileName, fields, stream)
}

// buildAudioMultipartRequest assembles a multipart/form-data request for STT endpoints,
// writing the audio file plus form fields, and a JSONBody placeholder for logging.
func (t *OutboundTransformer) buildAudioMultipartRequest(
	ctx context.Context,
	path string,
	requestType llm.RequestType,
	apiFormat llm.APIFormat,
	file []byte,
	fileName string,
	fields map[string][]string,
	stream bool,
) (*httpclient.Request, error) {
	fileName = sanitizeAudioFileName(fileName)
	if fileName == "" {
		fileName = "audio.mp3"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// CreateFormFile escapes quotes/backslashes in the filename; combined with
	// sanitizeAudioFileName (strips control chars) this prevents header injection
	// from client-controlled file names.
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(file)); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	for k, values := range fields {
		for _, v := range values {
			if err := writer.WriteField(k, v); err != nil {
				return nil, fmt.Errorf("failed to write %s field: %w", k, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build JSONBody for logging: replace binary audio with a size placeholder.
	jsonBody := make(map[string]any, len(fields)+1)

	for k, values := range fields {
		if len(values) == 1 {
			jsonBody[k] = values[0]
		} else {
			jsonBody[k] = values
		}
	}

	jsonBody["file"] = fmt.Sprintf("<audio bytes: %d, filename: %s>", len(file), fileName)

	jsonBodyBytes, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", writer.FormDataContentType())
	if stream {
		headers.Set("Accept", "text/event-stream")
	} else {
		headers.Set("Accept", "application/json")
	}

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         t.buildAudioURL(path),
		Headers:     headers,
		ContentType: writer.FormDataContentType(),
		Body:        body.Bytes(),
		JSONBody:    jsonBodyBytes,
		Auth: &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: t.config.APIKeyProvider.Get(ctx),
		},
		RequestType: string(requestType),
		APIFormat:   string(apiFormat),
	}, nil
}

// sanitizeAudioFileName strips control characters (CR/LF etc.) from a client-supplied
// filename so it cannot break the multipart Content-Disposition header.
func sanitizeAudioFileName(name string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}

		return r
	}, name)
}

// buildAudioURL builds the audio endpoint URL, honoring a custom EndpointPath override.
func (t *OutboundTransformer) buildAudioURL(defaultPath string) string {
	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath
	}

	return t.config.BaseURL + defaultPath
}

// transformSpeechStreamChunk parses one SSE event from a streaming TTS response.
// Lines like `data: {...}` have already been split by the SSE decoder; the Data field
// is the raw JSON payload of the event.
func transformSpeechStreamChunk(event *httpclient.StreamEvent) (*llm.Response, error) {
	if event == nil || len(event.Data) == 0 {
		//nolint:nilnil // skip empty heartbeats
		return nil, nil
	}

	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return llm.DoneResponse, nil
	}

	// Reuse the chat-stream error-event detector so upstream `event: error` or
	// `{"error":{...}}` payloads arriving after a 200 are surfaced as stream errors
	// rather than silently decoded into empty audio events.
	if streamErr := parseStreamErrorEvent(event); streamErr != nil {
		return nil, streamErr
	}

	var ev llm.SpeechStreamEvent
	if err := json.Unmarshal(event.Data, &ev); err != nil {
		return nil, fmt.Errorf("failed to decode speech stream event: %w", err)
	}

	return &llm.Response{
		RequestType:       llm.RequestTypeSpeech,
		APIFormat:         llm.APIFormatOpenAISpeech,
		SpeechStreamEvent: &ev,
		Usage:             ev.Usage,
	}, nil
}

func transformSpeechBinaryChunk(event *httpclient.StreamEvent) (*llm.Response, error) {
	if event != nil && event.Type == httpclient.BinaryStreamDoneEventType {
		return &llm.Response{
			Object:      "[DONE]",
			RequestType: llm.RequestTypeSpeech,
			APIFormat:   llm.APIFormatOpenAISpeech,
		}, nil
	}

	if event == nil || len(event.Data) == 0 {
		//nolint:nilnil // skip empty chunks
		return nil, nil
	}

	contentType := strings.TrimSpace(event.Type)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &llm.Response{
		RequestType: llm.RequestTypeSpeech,
		APIFormat:   llm.APIFormatOpenAISpeech,
		SpeechAudioChunk: &llm.SpeechAudioChunk{
			Audio:       bytes.Clone(event.Data),
			ContentType: contentType,
		},
	}, nil
}

func isSpeechBinaryStreamEvent(event *httpclient.StreamEvent) bool {
	return event != nil &&
		(event.Type == httpclient.BinaryStreamDoneEventType || event.IsBinaryAudioChunk())
}

// transformTranscriptionStreamChunkFor returns a decoder for streaming STT/translation SSE events.
func transformTranscriptionStreamChunkFor(apiFormat llm.APIFormat) func(*httpclient.StreamEvent) (*llm.Response, error) {
	requestType := requestTypeForAudioFormat(apiFormat)

	return func(event *httpclient.StreamEvent) (*llm.Response, error) {
		if event == nil || len(event.Data) == 0 {
			//nolint:nilnil // skip empty heartbeats
			return nil, nil
		}

		if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
			return llm.DoneResponse, nil
		}

		if streamErr := parseStreamErrorEvent(event); streamErr != nil {
			return nil, streamErr
		}

		var ev llm.TranscriptionStreamEvent
		if err := json.Unmarshal(event.Data, &ev); err != nil {
			return nil, fmt.Errorf("failed to decode transcription stream event: %w", err)
		}

		return &llm.Response{
			RequestType:              requestType,
			APIFormat:                apiFormat,
			TranscriptionStreamEvent: &ev,
			Usage:                    ev.Usage,
		}, nil
	}
}

func speechStreamChunkTransformFor(_ *httpclient.Request) func(*httpclient.StreamEvent) (*llm.Response, error) {
	return func(event *httpclient.StreamEvent) (*llm.Response, error) {
		if isSpeechBinaryStreamEvent(event) {
			return transformSpeechBinaryChunk(event)
		}

		return transformSpeechStreamChunk(event)
	}
}

// transformSpeechResponse transforms the binary audio response into a unified llm.Response.
// The audio bytes are passed through untouched.
func transformSpeechResponse(httpResp *httpclient.Response) (*llm.Response, error) {
	contentType := httpResp.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}

	return &llm.Response{
		RequestType: llm.RequestTypeSpeech,
		APIFormat:   llm.APIFormatOpenAISpeech,
		Speech: &llm.SpeechResponse{
			Audio:       httpResp.Body,
			ContentType: contentType,
		},
	}, nil
}

// transformTranscriptionResponse transforms the STT response into a unified llm.Response.
// JSON formats (json/verbose_json) are parsed; other formats (text/srt/vtt) are passed through raw.
func transformTranscriptionResponse(httpResp *httpclient.Response, apiFormat llm.APIFormat) (*llm.Response, error) {
	contentType := httpResp.Headers.Get("Content-Type")

	resp := &llm.Response{
		RequestType: requestTypeForAudioFormat(apiFormat),
		APIFormat:   apiFormat,
	}

	// Strictly parse only when the provider declares JSON; otherwise sniff with
	// json.Valid so plain-text transcripts starting with '[' or '{' (e.g. "[Music] ...")
	// are not misclassified and fed to json.Unmarshal.
	isJSON := strings.Contains(strings.ToLower(contentType), "application/json")
	if !isJSON && contentType == "" {
		isJSON = looksLikeJSON(httpResp.Body) && json.Valid(httpResp.Body)
	}

	if isJSON {
		var parsed llm.TranscriptionResponse
		if err := json.Unmarshal(httpResp.Body, &parsed); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transcription response: %w", err)
		}

		// Preserve the raw JSON so extra fields (segments, words, task for verbose_json)
		// are returned to the client untouched.
		parsed.Raw = httpResp.Body
		parsed.RawContentType = "application/json"
		resp.Transcription = &parsed

		return resp, nil
	}

	// Non-JSON format (text/srt/vtt): pass through raw.
	if contentType == "" {
		contentType = "text/plain"
	}

	resp.Transcription = &llm.TranscriptionResponse{
		Text:           string(httpResp.Body),
		Raw:            httpResp.Body,
		RawContentType: contentType,
	}

	return resp, nil
}

func requestTypeForAudioFormat(apiFormat llm.APIFormat) llm.RequestType {
	if apiFormat == llm.APIFormatOpenAITranslation {
		return llm.RequestTypeTranslation
	}

	return llm.RequestTypeTranscription
}

func looksLikeJSON(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}
