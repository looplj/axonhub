package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func newAudioOutbound(t *testing.T) *OutboundTransformer {
	t.Helper()

	out, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        "https://api.openai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("sk-test"),
	})
	require.NoError(t, err)

	return out.(*OutboundTransformer)
}

func TestOutbound_BuildSpeechRequest(t *testing.T) {
	out := newAudioOutbound(t)

	speed := 1.5
	stream := false
	llmReq := &llm.Request{
		Model:       "tts-1",
		Stream:      &stream,
		RequestType: llm.RequestTypeSpeech,
		APIFormat:   llm.APIFormatOpenAISpeech,
		Speech: &llm.SpeechRequest{
			Input:          "Hello",
			Voice:          "nova",
			ResponseFormat: "wav",
			Speed:          &speed,
		},
	}

	httpReq, err := out.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, httpReq.Method)
	require.Equal(t, "https://api.openai.com/v1/audio/speech", httpReq.URL)
	require.Equal(t, string(llm.RequestTypeSpeech), httpReq.RequestType)
	require.Equal(t, string(llm.APIFormatOpenAISpeech), httpReq.APIFormat)

	var body SpeechRequestBody
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))
	require.Equal(t, "tts-1", body.Model)
	require.Equal(t, "Hello", body.Input)
	require.Equal(t, "nova", body.Voice)
	require.Equal(t, "wav", body.ResponseFormat)
	require.NotNil(t, body.Speed)
	require.InDelta(t, 1.5, *body.Speed, 1e-9)

	var rawBody map[string]any
	require.NoError(t, json.Unmarshal(httpReq.Body, &rawBody))
	require.NotContains(t, rawBody, "stream")
}

func TestOutbound_BuildTranscriptionRequest(t *testing.T) {
	out := newAudioOutbound(t)

	llmReq := &llm.Request{
		Model:       "whisper-1",
		RequestType: llm.RequestTypeTranscription,
		APIFormat:   llm.APIFormatOpenAITranscription,
		Transcription: &llm.TranscriptionRequest{
			File:           []byte("AUDIO_DATA"),
			FileName:       "a.mp3",
			Language:       "en",
			ResponseFormat: "json",
		},
	}

	httpReq, err := out.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1/audio/transcriptions", httpReq.URL)

	mediaType, params, err := mime.ParseMediaType(httpReq.Headers.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])

	// Raw audio is in the wire body, but JSONBody (for logging) replaces it with a placeholder.
	require.Contains(t, string(httpReq.Body), "AUDIO_DATA")
	require.NotEmpty(t, httpReq.JSONBody)
	require.NotContains(t, string(httpReq.JSONBody), "AUDIO_DATA")
	require.Contains(t, string(httpReq.JSONBody), "audio bytes")
}

func TestOutbound_BuildTranscriptionRequest_ExtraFieldsAndFileName(t *testing.T) {
	out := newAudioOutbound(t)

	llmReq := &llm.Request{
		Model:       "whisper-1",
		RequestType: llm.RequestTypeTranscription,
		APIFormat:   llm.APIFormatOpenAITranscription,
		Transcription: &llm.TranscriptionRequest{
			File: []byte("AUDIO"),
			// Malicious filename attempting multipart header injection.
			FileName: "a\r\nContent-Type: text/evil\r\n.mp3",
			Extra: map[string][]string{
				"timestamp_granularities[]": {"word", "segment"},
			},
		},
	}

	httpReq, err := out.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	_, params, err := mime.ParseMediaType(httpReq.Headers.Get("Content-Type"))
	require.NoError(t, err)

	// Parse the generated multipart body back and verify fields/filename.
	reader := multipart.NewReader(bytes.NewReader(httpReq.Body), params["boundary"])
	form, err := reader.ReadForm(1 << 20)
	require.NoError(t, err)

	defer func() { _ = form.RemoveAll() }()

	require.Equal(t, []string{"word", "segment"}, form.Value["timestamp_granularities[]"])
	require.Len(t, form.File["file"], 1)
	// The multipart body parses cleanly (the injected header did not break it) and
	// no raw CR/LF from the filename leaked into the generated body.
	require.NotContains(t, string(httpReq.Body), "\r\nContent-Type: text/evil")
}

func TestOutbound_BuildTranslationRequest_URL(t *testing.T) {
	out := newAudioOutbound(t)

	httpReq, err := out.TransformRequest(context.Background(), &llm.Request{
		Model:       "whisper-1",
		RequestType: llm.RequestTypeTranslation,
		APIFormat:   llm.APIFormatOpenAITranslation,
		Translation: &llm.TranslationRequest{File: []byte("x"), FileName: "a.mp3"},
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1/audio/translations", httpReq.URL)
	require.Equal(t, string(llm.APIFormatOpenAITranslation), httpReq.APIFormat)
}

func TestOutbound_TransformSpeechResponse(t *testing.T) {
	out := newAudioOutbound(t)

	audio := []byte{0x00, 0x01, 0x02, 0x03}
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       audio,
		Headers:    http.Header{"Content-Type": []string{"audio/wav"}},
		Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAISpeech)},
	}

	llmResp, err := out.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp.Speech)
	require.Equal(t, audio, llmResp.Speech.Audio)
	require.Equal(t, "audio/wav", llmResp.Speech.ContentType)
}

func TestOutbound_TransformTranscriptionResponse(t *testing.T) {
	out := newAudioOutbound(t)

	t.Run("json", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"text":"hello world"}`),
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
			Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)},
		}

		llmResp, err := out.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp.Transcription)
		require.Equal(t, "hello world", llmResp.Transcription.Text)
		// Raw JSON is preserved for lossless passthrough back to the client.
		require.Equal(t, httpResp.Body, llmResp.Transcription.Raw)
		require.Equal(t, "application/json", llmResp.Transcription.RawContentType)
	})

	t.Run("verbose_json keeps raw body", func(t *testing.T) {
		raw := []byte(`{"task":"transcribe","language":"en","duration":1.5,"text":"hi","segments":[{"id":0}]}`)
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       raw,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
			Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)},
		}

		llmResp, err := out.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp.Transcription)
		require.Equal(t, raw, llmResp.Transcription.Raw)
	})

	t.Run("text response starting with bracket is not parsed as json", func(t *testing.T) {
		// response_format=text can yield transcripts like "[Music] hello" which start
		// with '[' but are not JSON; they must pass through raw instead of failing.
		raw := "[Music] hello world"
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(raw),
			Headers:    http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)},
		}

		llmResp, err := out.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp.Transcription)
		require.Equal(t, raw, string(llmResp.Transcription.Raw))
	})

	t.Run("missing content type with invalid json passes through raw", func(t *testing.T) {
		raw := "{not json at all"
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(raw),
			Headers:    http.Header{},
			Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)},
		}

		llmResp, err := out.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp.Transcription)
		require.Equal(t, raw, string(llmResp.Transcription.Raw))
	})

	t.Run("missing content type with valid json is parsed", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"text":"hi"}`),
			Headers:    http.Header{},
			Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)},
		}

		llmResp, err := out.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.Equal(t, "hi", llmResp.Transcription.Text)
	})

	t.Run("raw srt", func(t *testing.T) {
		raw := "1\n00:00:00,000 --> 00:00:01,000\nhi\n"
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(raw),
			Headers:    http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)},
		}

		llmResp, err := out.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp.Transcription)
		require.Equal(t, raw, string(llmResp.Transcription.Raw))
	})
}

// TestAudioRoundTrip verifies an end-to-end inbound->outbound->provider->outbound->inbound
// flow keeps the audio bytes intact for TTS.
func TestAudioRoundTrip_Speech(t *testing.T) {
	inbound := NewSpeechInboundTransformer()
	outbound := newAudioOutbound(t)

	clientBody, _ := json.Marshal(map[string]any{
		"model": "tts-1", "input": "Hi", "voice": "alloy",
	})

	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Body:    clientBody,
		Headers: http.Header{"Content-Type": []string{"application/json"}},
	})
	require.NoError(t, err)

	providerReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(providerReq.URL, "/audio/speech"))

	// Simulate the provider returning binary audio.
	audio := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	providerResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       audio,
		Headers:    http.Header{"Content-Type": []string{"audio/mpeg"}},
		Request:    providerReq,
	}

	llmResp, err := outbound.TransformResponse(context.Background(), providerResp)
	require.NoError(t, err)

	clientResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)
	require.Equal(t, audio, clientResp.Body)
	require.Equal(t, "audio/mpeg", clientResp.Headers.Get("Content-Type"))
}

// TestAudioStreaming_Speech verifies an end-to-end SSE streaming TTS round trip.
func TestAudioStreaming_Speech(t *testing.T) {
	inbound := NewSpeechInboundTransformer()
	outbound := newAudioOutbound(t)

	// Client requests stream_format=sse.
	clientBody, _ := json.Marshal(map[string]any{
		"model": "gpt-4o-mini-tts", "input": "Hi", "voice": "alloy",
		"stream_format": "sse",
	})

	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Body:    clientBody,
		Headers: http.Header{"Content-Type": []string{"application/json"}},
	})
	require.NoError(t, err)
	require.True(t, *llmReq.Stream)

	providerReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.Equal(t, "text/event-stream", providerReq.Headers.Get("Accept"))

	// stream_format passed through to the provider body.
	var providerBody map[string]any
	require.NoError(t, json.Unmarshal(providerReq.Body, &providerBody))
	require.Equal(t, "sse", providerBody["stream_format"])

	// Simulate the provider's SSE events flowing through the outbound transformer.
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"speech.audio.delta","audio":"YWJjZA=="}`)}, // "abcd" → 4 bytes
		{Data: []byte(`{"type":"speech.audio.delta","audio":"ZWZnaA=="}`)}, // "efgh" → 4 bytes
		{Data: []byte(`{"type":"speech.audio.done","usage":{"total_tokens":12}}`)},
		{Data: []byte("[DONE]")},
	}
	upstream := streams.SliceStream(events)

	llmStream, err := outbound.TransformStream(context.Background(), providerReq, upstream)
	require.NoError(t, err)

	clientStream, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	collected := make([]string, 0)

	var lastTyped *httpclient.StreamEvent

	for clientStream.Next() {
		ev := clientStream.Current()
		require.NotNil(t, ev)
		collected = append(collected, string(ev.Data))
		lastTyped = ev
	}

	require.NoError(t, clientStream.Err())
	// 3 events forwarded (delta, delta, done); [DONE] sentinel swallowed by inbound.
	require.Len(t, collected, 3)
	require.Contains(t, collected[0], `"speech.audio.delta"`)
	require.Contains(t, collected[2], `"speech.audio.done"`)

	// Terminal *.done event must carry its Type so the persistence layer can detect completion.
	require.NotNil(t, lastTyped)
	require.Equal(t, "speech.audio.done", lastTyped.Type)

	// Aggregation should track audio byte count and surface usage.
	body, meta, err := inbound.AggregateStreamChunks(context.Background(), events)
	require.NoError(t, err)
	require.Contains(t, string(body), `"audio_bytes":8`)
	require.True(t, meta.Completed)
	require.NotNil(t, meta.Usage)
	require.EqualValues(t, 12, meta.Usage.TotalTokens)
}

func TestAudioStreaming_SpeechBinaryChunks(t *testing.T) {
	inbound := NewSpeechInboundTransformer()
	outbound := newAudioOutbound(t)

	clientBody, _ := json.Marshal(map[string]any{
		"model": "gpt-4o-mini-tts", "input": "Hi", "voice": "alloy",
		"stream_format": "audio",
	})

	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Body:    clientBody,
		Headers: http.Header{"Content-Type": []string{"application/json"}},
	})
	require.NoError(t, err)
	require.NotNil(t, llmReq.Stream)
	require.True(t, *llmReq.Stream)

	providerReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.Equal(t, "*/*", providerReq.Headers.Get("Accept"))

	var providerBody map[string]any
	require.NoError(t, json.Unmarshal(providerReq.Body, &providerBody))
	require.Equal(t, "audio", providerBody["stream_format"])
	require.NotContains(t, providerBody, "stream")

	events := []*httpclient.StreamEvent{
		{Type: "audio/mpeg", Data: []byte{0x7b, 0x01, 0x02}},
		{Type: "audio/mpeg", Data: []byte{0x04, 0x05}},
		{Type: httpclient.BinaryStreamDoneEventType},
	}

	llmStream, err := outbound.TransformStream(context.Background(), providerReq, streams.SliceStream(events))
	require.NoError(t, err)

	clientStream, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var chunks []*httpclient.StreamEvent
	for clientStream.Next() {
		chunks = append(chunks, clientStream.Current())
	}
	require.NoError(t, clientStream.Err())
	require.Len(t, chunks, 3)
	require.Equal(t, "audio/mpeg", chunks[0].Type)
	require.Equal(t, []byte{0x7b, 0x01, 0x02}, chunks[0].Data)
	require.Equal(t, []byte{0x04, 0x05}, chunks[1].Data)
	require.Equal(t, httpclient.BinaryStreamDoneEventType, chunks[2].Type)

	body, meta, err := inbound.AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)
	require.Contains(t, string(body), `"audio_bytes":5`)
	require.Equal(t, llm.SpeechStreamResponseID, meta.ID)
	require.True(t, meta.Completed)
	require.Nil(t, meta.Usage)
}

// TestAudioStreaming_Transcription verifies an end-to-end SSE streaming STT round trip.
func TestAudioStreaming_Transcription(t *testing.T) {
	inbound := NewTranscriptionInboundTransformer()
	outbound := newAudioOutbound(t)

	// Multipart upload with stream=true.
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	part, err := writer.CreateFormFile("file", "a.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("AUDIO"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("model", "gpt-4o-transcribe"))
	require.NoError(t, writer.WriteField("stream", "true"))
	require.NoError(t, writer.Close())

	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Body:    buf.Bytes(),
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
	})
	require.NoError(t, err)
	require.True(t, *llmReq.Stream)
	// stream consumed by gateway, must not leak into Extra (would be forwarded twice).
	require.NotContains(t, llmReq.Transcription.Extra, "stream")

	providerReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.Equal(t, "text/event-stream", providerReq.Headers.Get("Accept"))
	// stream=true must appear once in the outbound multipart body.
	require.Equal(t, 1, strings.Count(string(providerReq.Body), `name="stream"`))

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"transcript.text.delta","delta":"hello "}`)},
		{Data: []byte(`{"type":"transcript.text.delta","delta":"world"}`)},
		{Data: []byte(`{"type":"transcript.text.done","text":"hello world","usage":{"total_tokens":5}}`)},
		{Data: []byte("[DONE]")},
	}

	llmStream, err := outbound.TransformStream(context.Background(), providerReq, streams.SliceStream(events))
	require.NoError(t, err)

	clientStream, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	collected := make([]string, 0)

	var lastTyped *httpclient.StreamEvent

	for clientStream.Next() {
		ev := clientStream.Current()
		collected = append(collected, string(ev.Data))
		lastTyped = ev
	}
	require.NoError(t, clientStream.Err())
	require.Len(t, collected, 3)
	require.Contains(t, collected[0], `"transcript.text.delta"`)
	require.Contains(t, collected[2], `"transcript.text.done"`)

	// Terminal *.done event must carry its Type so the persistence layer can detect completion.
	require.NotNil(t, lastTyped)
	require.Equal(t, "transcript.text.done", lastTyped.Type)

	body, meta, err := inbound.AggregateStreamChunks(context.Background(), events)
	require.NoError(t, err)
	require.Contains(t, string(body), `"text":"hello world"`)
	require.True(t, meta.Completed)
	require.NotNil(t, meta.Usage)
	require.EqualValues(t, 5, meta.Usage.TotalTokens)
}

// TestAudioStreaming_PropagatesErrorEvent verifies upstream stream-level errors arriving
// after a 200 are surfaced as stream errors instead of being decoded as empty audio events.
func TestAudioStreaming_PropagatesErrorEvent(t *testing.T) {
	outbound := newAudioOutbound(t)

	t.Run("speech", func(t *testing.T) {
		req := &httpclient.Request{APIFormat: string(llm.APIFormatOpenAISpeech)}
		events := []*httpclient.StreamEvent{
			{Data: []byte(`{"type":"speech.audio.delta","audio":"YWJjZA=="}`)},
			{Type: "error", Data: []byte(`{"error":{"message":"upstream broke","type":"server_error"}}`)},
		}

		llmStream, err := outbound.TransformStream(context.Background(), req, streams.SliceStream(events))
		require.NoError(t, err)

		var lastErr error
		for llmStream.Next() {
			_ = llmStream.Current()
		}
		lastErr = llmStream.Err()
		require.Error(t, lastErr)
		require.Contains(t, lastErr.Error(), "upstream broke")
	})

	t.Run("transcription", func(t *testing.T) {
		req := &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)}
		events := []*httpclient.StreamEvent{
			{Data: []byte(`{"type":"transcript.text.delta","delta":"hi"}`)},
			{Data: []byte(`{"error":{"message":"rate limited","code":"rate_limit"}}`)},
		}

		llmStream, err := outbound.TransformStream(context.Background(), req, streams.SliceStream(events))
		require.NoError(t, err)

		for llmStream.Next() {
			_ = llmStream.Current()
		}
		require.Error(t, llmStream.Err())
		require.Contains(t, llmStream.Err().Error(), "rate limited")
	})
}
