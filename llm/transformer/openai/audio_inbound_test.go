package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestAudioInboundTransformer_Speech(t *testing.T) {
	tr := NewSpeechInboundTransformer()
	require.Equal(t, llm.APIFormatOpenAISpeech, tr.APIFormat())

	t.Run("valid request", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model":           "tts-1",
			"input":           "Hello world",
			"voice":           "alloy",
			"response_format": "mp3",
			"speed":           1.25,
		})
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		}

		llmReq, err := tr.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.Equal(t, "tts-1", llmReq.Model)
		require.Equal(t, llm.RequestTypeSpeech, llmReq.RequestType)
		require.Equal(t, llm.APIFormatOpenAISpeech, llmReq.APIFormat)
		require.NotNil(t, llmReq.Speech)
		require.Equal(t, "Hello world", llmReq.Speech.Input)
		require.Equal(t, "alloy", llmReq.Speech.Voice)
		require.Equal(t, "mp3", llmReq.Speech.ResponseFormat)
		require.NotNil(t, llmReq.Speech.Speed)
		require.InDelta(t, 1.25, *llmReq.Speech.Speed, 1e-9)
	})

	t.Run("missing input", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"model": "tts-1"})
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.Error(t, err)
	})

	t.Run("missing voice", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"model": "tts-1", "input": "hello"})
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "voice")
	})

	t.Run("empty body", func(t *testing.T) {
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{})
		require.Error(t, err)
	})

	t.Run("response binary passthrough", func(t *testing.T) {
		audio := []byte{0x49, 0x44, 0x33, 0x04, 0x00}
		resp, err := tr.TransformResponse(context.Background(), &llm.Response{
			Speech: &llm.SpeechResponse{Audio: audio, ContentType: "audio/mpeg"},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, audio, resp.Body)
		require.Equal(t, "audio/mpeg", resp.Headers.Get("Content-Type"))
	})

	t.Run("unsupported stream_format rejected", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model": "gpt-4o-mini-tts", "input": "hi", "voice": "alloy",
			"stream_format": "json",
		})
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "stream_format")
	})

	t.Run("sse stream_format enables streaming", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model": "gpt-4o-mini-tts", "input": "hi", "voice": "alloy",
			"stream_format": "sse",
		})
		req, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.NoError(t, err)
		require.NotNil(t, req.Stream)
		require.True(t, *req.Stream)
		require.Equal(t, "sse", req.Speech.StreamFormat)
	})

	t.Run("audio stream_format enables binary streaming", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model": "gpt-4o-mini-tts", "input": "hi", "voice": "alloy",
			"stream_format": "audio",
		})
		req, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.NoError(t, err)
		require.NotNil(t, req.Stream)
		require.True(t, *req.Stream)
		require.Equal(t, "audio", req.Speech.StreamFormat)
	})

	t.Run("missing stream_format stays non-streaming", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model": "gpt-4o-mini-tts", "input": "hi", "voice": "alloy",
		})
		req, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.NoError(t, err)
		require.NotNil(t, req.Stream)
		require.False(t, *req.Stream)
		require.Empty(t, req.Speech.StreamFormat)
	})
}

func TestAudioInboundTransformer_Transcription(t *testing.T) {
	tr := NewTranscriptionInboundTransformer()
	require.Equal(t, llm.APIFormatOpenAITranscription, tr.APIFormat())

	t.Run("valid multipart request", func(t *testing.T) {
		audio := []byte("FAKE_AUDIO_BYTES")
		contentType, body := buildMultipartAudio(t, audio, "speech.mp3", map[string]string{
			"model":           "whisper-1",
			"language":        "en",
			"response_format": "json",
			"temperature":     "0.2",
		})

		httpReq := &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{contentType}},
		}

		llmReq, err := tr.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.Equal(t, "whisper-1", llmReq.Model)
		require.Equal(t, llm.RequestTypeTranscription, llmReq.RequestType)
		require.NotNil(t, llmReq.Transcription)
		require.Equal(t, audio, llmReq.Transcription.File)
		require.Equal(t, "speech.mp3", llmReq.Transcription.FileName)
		require.Equal(t, "en", llmReq.Transcription.Language)
		require.Equal(t, "json", llmReq.Transcription.ResponseFormat)
		require.NotNil(t, llmReq.Transcription.Temperature)
		require.InDelta(t, 0.2, *llmReq.Transcription.Temperature, 1e-9)

		// JSONBody should be populated for logging and must not contain the raw audio.
		require.NotEmpty(t, httpReq.JSONBody)
		require.NotContains(t, string(httpReq.JSONBody), "FAKE_AUDIO_BYTES")
	})

	t.Run("missing file", func(t *testing.T) {
		contentType, body := buildMultipartAudio(t, nil, "", map[string]string{"model": "whisper-1"})
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{contentType}},
		})
		require.Error(t, err)
	})

	t.Run("duplicate file parts rejected", func(t *testing.T) {
		buf := &bytes.Buffer{}
		writer := multipart.NewWriter(buf)

		for _, name := range []string{"a.mp3", "b.mp3"} {
			part, err := writer.CreateFormFile("file", name)
			require.NoError(t, err)
			_, err = part.Write([]byte("AUDIO"))
			require.NoError(t, err)
		}

		require.NoError(t, writer.WriteField("model", "whisper-1"))
		require.NoError(t, writer.Close())

		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    buf.Bytes(),
			Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "multiple file")
	})

	t.Run("extra fields preserved", func(t *testing.T) {
		audio := []byte("AUDIO")
		buf := &bytes.Buffer{}
		writer := multipart.NewWriter(buf)

		part, err := writer.CreateFormFile("file", "a.mp3")
		require.NoError(t, err)
		_, err = part.Write(audio)
		require.NoError(t, err)

		require.NoError(t, writer.WriteField("model", "whisper-1"))
		require.NoError(t, writer.WriteField("response_format", "verbose_json"))
		// Repeated unmodeled field must keep multiplicity.
		require.NoError(t, writer.WriteField("timestamp_granularities[]", "word"))
		require.NoError(t, writer.WriteField("timestamp_granularities[]", "segment"))
		require.NoError(t, writer.Close())

		llmReq, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    buf.Bytes(),
			Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		})
		require.NoError(t, err)
		require.Equal(t, map[string][]string{
			"timestamp_granularities[]": {"word", "segment"},
		}, llmReq.Transcription.Extra)
	})

	t.Run("invalid temperature", func(t *testing.T) {
		contentType, body := buildMultipartAudio(t, []byte("AUDIO"), "a.mp3", map[string]string{
			"model":       "whisper-1",
			"temperature": "abc",
		})
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    body,
			Headers: http.Header{"Content-Type": []string{contentType}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "temperature")
	})

	t.Run("not multipart", func(t *testing.T) {
		_, err := tr.TransformRequest(context.Background(), &httpclient.Request{
			Body:    []byte(`{"model":"whisper-1"}`),
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		})
		require.Error(t, err)
	})

	t.Run("response json", func(t *testing.T) {
		resp, err := tr.TransformResponse(context.Background(), &llm.Response{
			Transcription: &llm.TranscriptionResponse{Text: "hello"},
		})
		require.NoError(t, err)
		require.Equal(t, "application/json", resp.Headers.Get("Content-Type"))

		var out map[string]any
		require.NoError(t, json.Unmarshal(resp.Body, &out))
		require.Equal(t, "hello", out["text"])
	})

	t.Run("response raw verbose_json passthrough keeps extra fields", func(t *testing.T) {
		raw := []byte(`{"task":"transcribe","text":"hello","segments":[{"id":0,"text":"hello"}]}`)
		resp, err := tr.TransformResponse(context.Background(), &llm.Response{
			Transcription: &llm.TranscriptionResponse{Text: "hello", Raw: raw, RawContentType: "application/json"},
		})
		require.NoError(t, err)
		require.Equal(t, raw, resp.Body)
		require.Equal(t, "application/json", resp.Headers.Get("Content-Type"))
	})

	t.Run("response raw srt passthrough", func(t *testing.T) {
		raw := []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n")
		resp, err := tr.TransformResponse(context.Background(), &llm.Response{
			Transcription: &llm.TranscriptionResponse{Raw: raw, RawContentType: "text/plain"},
		})
		require.NoError(t, err)
		require.Equal(t, raw, resp.Body)
		require.Equal(t, "text/plain", resp.Headers.Get("Content-Type"))
	})
}

func TestAudioInboundTransformer_Translation(t *testing.T) {
	tr := NewTranslationInboundTransformer()
	require.Equal(t, llm.APIFormatOpenAITranslation, tr.APIFormat())

	audio := []byte("FAKE_AUDIO")
	contentType, body := buildMultipartAudio(t, audio, "de.mp3", map[string]string{"model": "whisper-1"})

	llmReq, err := tr.TransformRequest(context.Background(), &httpclient.Request{
		Body:    body,
		Headers: http.Header{"Content-Type": []string{contentType}},
	})
	require.NoError(t, err)
	require.Equal(t, llm.RequestTypeTranslation, llmReq.RequestType)
	require.NotNil(t, llmReq.Translation)
	require.Equal(t, audio, llmReq.Translation.File)
	require.Equal(t, "de.mp3", llmReq.Translation.FileName)
}

// buildMultipartAudio builds a multipart/form-data body with an optional file part and fields.
func buildMultipartAudio(t *testing.T, file []byte, filename string, fields map[string]string) (string, []byte) {
	t.Helper()

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)

	if len(file) > 0 {
		part, err := writer.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = part.Write(file)
		require.NoError(t, err)
	}

	for k, v := range fields {
		require.NoError(t, writer.WriteField(k, v))
	}

	require.NoError(t, writer.Close())

	return writer.FormDataContentType(), buf.Bytes()
}
