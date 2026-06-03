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
	llmReq := &llm.Request{
		Model:       "tts-1",
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
