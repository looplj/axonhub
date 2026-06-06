package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	transformer "github.com/looplj/axonhub/llm/transformer"
)

const (
	// maxAudioBodySize bounds the raw multipart body for STT uploads.
	maxAudioBodySize = 50 * 1024 * 1024
	// maxAudioFileSize bounds a single audio file (OpenAI Whisper limit is 25MB).
	maxAudioFileSize = 26 * 1024 * 1024
)

// AudioInboundTransformer implements transformer.Inbound for OpenAI audio APIs
// (text-to-speech and speech-to-text).
type AudioInboundTransformer struct {
	apiFormat llm.APIFormat
}

// NewSpeechInboundTransformer creates an inbound transformer for POST /v1/audio/speech (TTS).
func NewSpeechInboundTransformer() *AudioInboundTransformer {
	return &AudioInboundTransformer{apiFormat: llm.APIFormatOpenAISpeech}
}

// NewTranscriptionInboundTransformer creates an inbound transformer for POST /v1/audio/transcriptions (STT).
func NewTranscriptionInboundTransformer() *AudioInboundTransformer {
	return &AudioInboundTransformer{apiFormat: llm.APIFormatOpenAITranscription}
}

// NewTranslationInboundTransformer creates an inbound transformer for POST /v1/audio/translations (STT).
func NewTranslationInboundTransformer() *AudioInboundTransformer {
	return &AudioInboundTransformer{apiFormat: llm.APIFormatOpenAITranslation}
}

// APIFormat returns the API format of the transformer.
func (t *AudioInboundTransformer) APIFormat() llm.APIFormat {
	return t.apiFormat
}

func (t *AudioInboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	//nolint:exhaustive // Only audio-related API formats are handled here.
	switch t.apiFormat {
	case llm.APIFormatOpenAISpeech:
		return t.transformSpeechRequest(httpReq)
	case llm.APIFormatOpenAITranscription:
		return t.transformTranscriptionRequest(httpReq)
	case llm.APIFormatOpenAITranslation:
		return t.transformTranslationRequest(httpReq)
	default:
		return nil, fmt.Errorf("%w: unknown audio api format: %s", transformer.ErrInvalidRequest, t.apiFormat)
	}
}

func (t *AudioInboundTransformer) transformSpeechRequest(httpReq *httpclient.Request) (*llm.Request, error) {
	contentType := strings.ToLower(httpReq.Headers.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("%w: speech requires application/json", transformer.ErrInvalidRequest)
	}

	var body SpeechRequestBody
	if err := json.Unmarshal(httpReq.Body, &body); err != nil {
		return nil, fmt.Errorf("%w: failed to decode speech request: %w", transformer.ErrInvalidRequest, err)
	}

	if body.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if body.Input == "" {
		return nil, fmt.Errorf("%w: input is required", transformer.ErrInvalidRequest)
	}

	if body.Voice == "" {
		return nil, fmt.Errorf("%w: voice is required", transformer.ErrInvalidRequest)
	}

	// stream_format is opt-in. When the client omits it, OpenAI returns a single
	// binary audio body (non-streaming) and we keep that path so the gateway can
	// persist the audio bytes to external storage via UpdateRequestCompletedWithAudio.
	// Only explicit "sse" or "audio" engages the streaming pipeline.
	streamFormat := strings.ToLower(strings.TrimSpace(body.StreamFormat))
	if streamFormat != "" && streamFormat != "sse" && streamFormat != "audio" {
		return nil, fmt.Errorf("%w: unsupported stream_format: %q (only \"sse\" and \"audio\" are supported)", transformer.ErrInvalidRequest, body.StreamFormat)
	}

	isStream := streamFormat != ""

	return &llm.Request{
		Model:       body.Model,
		Stream:      lo.ToPtr(isStream),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeSpeech,
		APIFormat:   t.apiFormat,
		Speech: &llm.SpeechRequest{
			Input:          body.Input,
			Voice:          body.Voice,
			ResponseFormat: body.ResponseFormat,
			Speed:          body.Speed,
			Instructions:   body.Instructions,
			StreamFormat:   streamFormat,
		},
	}, nil
}

func (t *AudioInboundTransformer) transformTranscriptionRequest(httpReq *httpclient.Request) (*llm.Request, error) {
	form, err := parseAudioMultipartRequest(httpReq)
	if err != nil {
		return nil, err
	}

	model := strings.TrimSpace(form.First("model"))
	if model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(form.File) == 0 {
		return nil, fmt.Errorf("%w: file is required for transcription", transformer.ErrInvalidRequest)
	}

	temperature, err := parseOptionalFloat64("temperature", form.First("temperature"))
	if err != nil {
		return nil, err
	}

	isStream, err := parseStreamField(form.First("stream"))
	if err != nil {
		return nil, err
	}

	extra := form.extraFields()
	// stream is consumed by the gateway; do not forward it twice via Extra.
	delete(extra, "stream")

	httpReq.JSONBody = buildAudioJSONBody(form)

	return &llm.Request{
		Model:       model,
		Stream:      lo.ToPtr(isStream),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeTranscription,
		APIFormat:   t.apiFormat,
		Transcription: &llm.TranscriptionRequest{
			File:           form.File,
			FileName:       form.FileName,
			Language:       strings.TrimSpace(form.First("language")),
			Prompt:         strings.TrimSpace(form.First("prompt")),
			ResponseFormat: strings.TrimSpace(form.First("response_format")),
			Temperature:    temperature,
			Extra:          extra,
		},
	}, nil
}

func (t *AudioInboundTransformer) transformTranslationRequest(httpReq *httpclient.Request) (*llm.Request, error) {
	form, err := parseAudioMultipartRequest(httpReq)
	if err != nil {
		return nil, err
	}

	model := strings.TrimSpace(form.First("model"))
	if model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(form.File) == 0 {
		return nil, fmt.Errorf("%w: file is required for translation", transformer.ErrInvalidRequest)
	}

	temperature, err := parseOptionalFloat64("temperature", form.First("temperature"))
	if err != nil {
		return nil, err
	}

	isStream, err := parseStreamField(form.First("stream"))
	if err != nil {
		return nil, err
	}

	extra := form.extraFields()
	delete(extra, "stream")

	httpReq.JSONBody = buildAudioJSONBody(form)

	return &llm.Request{
		Model:       model,
		Stream:      lo.ToPtr(isStream),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeTranslation,
		APIFormat:   t.apiFormat,
		Translation: &llm.TranslationRequest{
			File:           form.File,
			FileName:       form.FileName,
			Prompt:         strings.TrimSpace(form.First("prompt")),
			ResponseFormat: strings.TrimSpace(form.First("response_format")),
			Temperature:    temperature,
			Extra:          extra,
		},
	}, nil
}

func (t *AudioInboundTransformer) TransformResponse(ctx context.Context, llmResp *llm.Response) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("%w: audio response is nil", transformer.ErrInvalidResponse)
	}

	if t.apiFormat == llm.APIFormatOpenAISpeech {
		if llmResp.Speech == nil {
			return nil, fmt.Errorf("%w: speech response is nil", transformer.ErrInvalidResponse)
		}

		contentType := llmResp.Speech.ContentType
		if contentType == "" {
			contentType = "audio/mpeg"
		}

		return &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       llmResp.Speech.Audio,
			Headers: http.Header{
				"Content-Type": []string{contentType},
			},
		}, nil
	}

	// Transcription / translation response.
	if llmResp.Transcription == nil {
		return nil, fmt.Errorf("%w: transcription response is nil", transformer.ErrInvalidResponse)
	}

	tr := llmResp.Transcription

	// When the provider raw body is available, pass it through untouched. This keeps
	// non-JSON formats (text/srt/vtt) intact and preserves all verbose_json fields
	// (segments, words, task) that the parsed struct does not model.
	if len(tr.Raw) > 0 {
		contentType := tr.RawContentType
		if contentType == "" {
			contentType = "application/json"
		}

		return &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       tr.Raw,
			Headers: http.Header{
				"Content-Type": []string{contentType},
			},
		}, nil
	}

	out := struct {
		Text     string   `json:"text"`
		Language string   `json:"language,omitempty"`
		Duration *float64 `json:"duration,omitempty"`
	}{
		Text:     tr.Text,
		Language: tr.Language,
		Duration: tr.Duration,
	}

	body, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transcription response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
}

// TransformStream converts unified streaming audio responses back into client-facing
// stream events. TTS supports both SSE and raw binary chunk streams; STT/translation
// use SSE only.
func (t *AudioInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.NoNil(streams.MapErr(stream, func(resp *llm.Response) (*httpclient.StreamEvent, error) {
		if resp == nil {
			//nolint:nilnil // skip nil chunks
			return nil, nil
		}

		// The DoneResponse sentinel is the pipeline-level end marker. SSE audio
		// streams have their own "*.done" events; binary speech streams need a
		// client-facing binary.done so persistence can mark the stream complete.
		if resp == llm.DoneResponse || resp.Object == "[DONE]" {
			if t.apiFormat == llm.APIFormatOpenAISpeech && resp.RequestType == llm.RequestTypeSpeech {
				return &httpclient.StreamEvent{Type: httpclient.BinaryStreamDoneEventType}, nil
			}

			//nolint:nilnil // skip pipeline sentinel
			return nil, nil
		}

		if resp.SpeechStreamEvent != nil {
			data, err := json.Marshal(resp.SpeechStreamEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal speech stream event: %w", err)
			}

			// Propagate the event type so InboundPersistentStream.isTerminalStreamEvent
			// can recognize speech.audio.done and mark the request as completed.
			return &httpclient.StreamEvent{Type: resp.SpeechStreamEvent.Type, Data: data}, nil
		}

		if resp.SpeechAudioChunk != nil {
			return &httpclient.StreamEvent{
				Type: resp.SpeechAudioChunk.ContentType,
				Data: resp.SpeechAudioChunk.Audio,
			}, nil
		}

		if resp.TranscriptionStreamEvent != nil {
			data, err := json.Marshal(resp.TranscriptionStreamEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal transcription stream event: %w", err)
			}

			return &httpclient.StreamEvent{Type: resp.TranscriptionStreamEvent.Type, Data: data}, nil
		}

		//nolint:nilnil // unrelated event, skip
		return nil, nil
	})), nil
}

func (t *AudioInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	chatInbound := NewInboundTransformer()
	return chatInbound.TransformError(ctx, rawErr)
}

// AggregateStreamChunks reassembles the streamed SSE events into a single JSON body for
// persistence/logging. For STT it produces the equivalent of a non-streamed JSON response;
// for TTS it returns a metadata summary (audio bytes are tracked separately via the
// content_storage_* fields and never written into the JSON column).
func (t *AudioInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if t.apiFormat == llm.APIFormatOpenAISpeech {
		return aggregateSpeechStreamChunks(chunks)
	}

	return aggregateTranscriptionStreamChunks(chunks)
}

func aggregateSpeechStreamChunks(chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	var (
		audioBytes int
		usage      *llm.Usage
		completed  bool
	)

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}

		if chunk.Type == httpclient.BinaryStreamDoneEventType {
			completed = true
			continue
		}

		if isSpeechBinaryStreamEvent(chunk) {
			// Persistence layer may have replaced the binary payload with a size
			// summary; fall back to chunk.Size when Data is empty.
			if n := len(chunk.Data); n > 0 {
				audioBytes += n
			} else {
				audioBytes += chunk.Size
			}

			continue
		}

		if len(chunk.Data) == 0 {
			continue
		}

		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			completed = true
			continue
		}

		var ev llm.SpeechStreamEvent
		if err := json.Unmarshal(chunk.Data, &ev); err != nil {
			continue
		}

		if ev.AudioBase64 != "" {
			// Estimate decoded byte count without actually decoding to keep aggregation cheap.
			audioBytes += base64DecodedLen(ev.AudioBase64)
		}

		if ev.Type == "speech.audio.done" {
			completed = true
		}

		if ev.Usage != nil {
			usage = ev.Usage
		}
	}

	body, err := json.Marshal(map[string]any{
		"object":      llm.SpeechStreamResponseID,
		"audio_bytes": audioBytes,
		"chunks":      len(chunks),
	})
	if err != nil {
		return nil, llm.ResponseMeta{}, fmt.Errorf("failed to marshal speech stream aggregate: %w", err)
	}

	meta := llm.ResponseMeta{
		ID:        llm.SpeechStreamResponseID,
		Usage:     usage,
		Completed: completed,
	}

	return body, meta, nil
}

func aggregateTranscriptionStreamChunks(chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	var (
		deltaBuilder strings.Builder
		finalText    string
		usage        *llm.Usage
		completed    bool
	)

	for _, chunk := range chunks {
		if chunk == nil || len(chunk.Data) == 0 {
			continue
		}

		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			completed = true
			continue
		}

		var ev llm.TranscriptionStreamEvent
		if err := json.Unmarshal(chunk.Data, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "transcript.text.delta":
			deltaBuilder.WriteString(ev.Delta)
		case "transcript.text.done":
			completed = true
			if ev.Text != "" {
				finalText = ev.Text
			}
			if ev.Usage != nil {
				usage = ev.Usage
			}
		}
	}

	text := finalText
	if text == "" {
		text = deltaBuilder.String()
	}

	body, err := json.Marshal(map[string]any{"text": text})
	if err != nil {
		return nil, llm.ResponseMeta{}, fmt.Errorf("failed to marshal transcription stream aggregate: %w", err)
	}

	return body, llm.ResponseMeta{Usage: usage, Completed: completed}, nil
}

// base64DecodedLen returns the decoded byte length of a standard base64 string
// (without allocating). It tolerates missing padding by ignoring trailing whitespace.
func base64DecodedLen(s string) int {
	n := len(strings.TrimRight(s, "="))
	return n * 3 / 4
}

// parseStreamField parses the multipart "stream" field; tolerant of common boolean spellings.
func parseStreamField(s string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false, nil
	}

	switch s {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%w: invalid stream value: %q", transformer.ErrInvalidRequest, s)
	}
}

// audioFormData holds the parsed multipart form of an STT request.
type audioFormData struct {
	File     []byte
	FileName string
	// Fields keeps multiplicity for repeated multipart fields (e.g. timestamp_granularities[]).
	Fields map[string][]string
}

// First returns the first value of a multipart field, or "" when absent.
func (f *audioFormData) First(name string) string {
	values := f.Fields[name]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// audioKnownFields are the multipart fields modeled explicitly on the unified request;
// all other fields are preserved in Extra and forwarded to the provider as-is.
var audioKnownFields = map[string]bool{
	"model":           true,
	"language":        true,
	"prompt":          true,
	"response_format": true,
	"temperature":     true,
}

// extraFields extracts unmodeled multipart fields (preserving multiplicity) for passthrough.
func (f *audioFormData) extraFields() map[string][]string {
	var extra map[string][]string

	for name, values := range f.Fields {
		if audioKnownFields[name] {
			continue
		}

		if extra == nil {
			extra = make(map[string][]string)
		}

		extra[name] = values
	}

	return extra
}

func parseAudioMultipartRequest(httpReq *httpclient.Request) (*audioFormData, error) {
	if len(httpReq.Body) > maxAudioBodySize {
		return nil, fmt.Errorf("%w: request body too large", transformer.ErrInvalidRequest)
	}

	mediaType, params, err := mime.ParseMediaType(httpReq.Headers.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid content-type", transformer.ErrInvalidRequest)
	}

	if !strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return nil, fmt.Errorf("%w: expected multipart/form-data", transformer.ErrInvalidRequest)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("%w: missing boundary in content-type", transformer.ErrInvalidRequest)
	}

	reader := multipart.NewReader(bytes.NewReader(httpReq.Body), boundary)
	form := &audioFormData{Fields: map[string][]string{}}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart", transformer.ErrInvalidRequest)
		}

		fieldName := part.FormName()
		filename := part.FileName()

		if filename == "" {
			value, err := io.ReadAll(io.LimitReader(part, maxAudioFileSize+1))
			if err != nil {
				return nil, fmt.Errorf("%w: failed to read multipart field", transformer.ErrInvalidRequest)
			}

			if len(value) > maxAudioFileSize {
				return nil, fmt.Errorf("%w: multipart field too large", transformer.ErrInvalidRequest)
			}

			form.Fields[fieldName] = append(form.Fields[fieldName], string(value))

			continue
		}

		if fieldName != "file" {
			continue
		}

		// Fail fast on duplicate file parts instead of silently using the last one.
		if form.File != nil {
			return nil, fmt.Errorf("%w: multiple file parts are not allowed", transformer.ErrInvalidRequest)
		}

		data, err := io.ReadAll(io.LimitReader(part, maxAudioFileSize+1))
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart file", transformer.ErrInvalidRequest)
		}

		if len(data) > maxAudioFileSize {
			return nil, fmt.Errorf("%w: file too large", transformer.ErrInvalidRequest)
		}

		form.File = data
		form.FileName = filename
	}

	return form, nil
}

// buildAudioJSONBody builds a JSON representation of the multipart request for logging,
// replacing the binary audio file with a size placeholder.
func buildAudioJSONBody(form *audioFormData) []byte {
	body := make(map[string]any, len(form.Fields)+1)

	for k, values := range form.Fields {
		switch len(values) {
		case 0:
		case 1:
			if values[0] != "" {
				body[k] = values[0]
			}
		default:
			body[k] = values
		}
	}

	body["file"] = fmt.Sprintf("<audio bytes: %d, filename: %s>", len(form.File), form.FileName)

	b, err := json.Marshal(body)
	if err != nil {
		return nil
	}

	return b
}

// parseOptionalFloat64 parses an optional float field; an invalid non-empty value is
// rejected (fail-fast) instead of being silently dropped.
func parseOptionalFloat64(name, s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid %s: %q", transformer.ErrInvalidRequest, name, s)
	}

	return &v, nil
}
