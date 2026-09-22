package modelmetadata

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestSentModel(t *testing.T) {
	tests := []struct {
		name    string
		format  llm.APIFormat
		request *httpclient.Request
		want    string
	}{
		{"nil", llm.APIFormatOpenAIChatCompletion, nil, ""},
		{"final JSON body overrides logging summary", llm.APIFormatOpenAIChatCompletion, &httpclient.Request{
			Body: []byte(`{"model":"sent-b"}`), JSONBody: []byte(`{"model":"logged-a"}`),
		}, "sent-b"},
		{"logging summary alone is not evidence", llm.APIFormatOpenAIChatCompletion, &httpclient.Request{JSONBody: []byte(`{"model":"logged-a"}`)}, ""},
		{"JSON media type with parameters", llm.APIFormatOpenAIResponse, &httpclient.Request{
			Body: []byte(`{"model":"response-model"}`), ContentType: "application/json; charset=utf-8",
		}, "response-model"},
		{"header content type", llm.APIFormatOpenAIChatCompletion, &httpclient.Request{
			Body: []byte(`{"model":"text"}`), Headers: http.Header{"Content-Type": {"text/plain"}},
		}, ""},
		{"actual content type wins over header", llm.APIFormatOpenAIChatCompletion, &httpclient.Request{
			Body: []byte(`{"model":"sent"}`), ContentType: "application/json", Headers: http.Header{"Content-Type": {"text/plain"}},
		}, "sent"},
		{"Gemini uses URL not body override", llm.APIFormatGeminiContents, &httpclient.Request{
			URL: "https://example.invalid/v1beta/models/gemini-a:generateContent?key=redacted", Body: []byte(`{"model":"unrelated-b"}`),
		}, "gemini-a"},
		{"actual Gemini format overrides chat fallback", llm.APIFormatOpenAIChatCompletion, &httpclient.Request{
			APIFormat: string(llm.APIFormatGeminiContents), URL: "https://example.invalid/v1beta/models/gemini-b:streamGenerateContent?alt=sse",
		}, "gemini-b"},
		{"Gemini image fallback when format omitted", llm.APIFormatGeminiContents, &httpclient.Request{
			URL: "https://example.invalid/v1beta/models/image-model:predict", RequestType: string(llm.RequestTypeImage),
		}, "image-model"},
		{"Gemini embedding URL", llm.APIFormatGeminiEmbedding, &httpclient.Request{URL: "https://example.invalid/v1beta/models/embed-model:embedContent"}, "embed-model"},
		{"Gemini batch embedding URL", llm.APIFormatGeminiEmbedding, &httpclient.Request{URL: "https://example.invalid/v1beta/models/embed-model:batchEmbedContents"}, "embed-model"},
		{"Antigravity envelope uses final transformed model", llm.APIFormatGeminiContents, &httpclient.Request{
			URL: "https://example.invalid/v1internal:generateContent", Metadata: map[string]string{"antigravity_model": "old-name"}, Body: []byte(`{"model":"final-name","request":{}}`),
		}, "final-name"},
		{"Bedrock URL model", llm.APIFormatAnthropicMessage, &httpclient.Request{
			URL: "https://example.invalid/model/anthropic.claude%3A0/invoke-with-response-stream", Body: []byte(`{"model":"ignored"}`),
		}, "anthropic.claude:0"},
		{"unsupported URL suffix", llm.APIFormatGeminiContents, &httpclient.Request{URL: "https://example.invalid/models/gemini-a:unknown"}, ""},
		{"unsupported protocol", "custom/unknown", &httpclient.Request{Body: []byte(`{"model":"not-supported"}`)}, ""},
		{"explicit unsupported format overrides fallback", llm.APIFormatOpenAIChatCompletion, &httpclient.Request{APIFormat: "custom/unknown", Body: []byte(`{"model":"not-supported"}`)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, SentModel(tt.request, tt.format)) })
	}
}

func TestSentModelMultipart(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   string
	}{
		{"model field", []string{"whisper-1"}, "whisper-1"},
		{"identical duplicate fields", []string{"whisper-1", "whisper-1"}, "whisper-1"},
		{"conflicting duplicate fields", []string{"whisper-1", "whisper-2"}, ""},
		{"missing field", nil, ""},
		{"too long", []string{strings.Repeat("a", maxModelBytes+1)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			file, err := writer.CreateFormFile("model", "audio.wav")
			require.NoError(t, err)
			_, err = file.Write([]byte(`{"model":"generated-content"}`))
			require.NoError(t, err)
			for _, model := range tt.models {
				require.NoError(t, writer.WriteField("model", model))
			}
			require.NoError(t, writer.Close())
			req := &httpclient.Request{Body: body.Bytes(), ContentType: writer.FormDataContentType(), JSONBody: []byte(`{"model":"logged-old"}`)}
			require.Equal(t, tt.want, SentModel(req, llm.APIFormatOpenAITranscription))
			require.Empty(t, SentModel(req, "unsupported/protocol"))
			req.Body = req.Body[:len(req.Body)-10]
			require.Empty(t, SentModel(req, llm.APIFormatOpenAITranscription), "truncated multipart is not conclusive metadata")
		})
	}
}

func TestResponseModel(t *testing.T) {
	tests := []struct {
		name        string
		format      llm.APIFormat
		body        string
		contentType string
		want        string
	}{
		{"OpenAI chat", llm.APIFormatOpenAIChatCompletion, `{"model":"provider-model","choices":[]}`, "application/json", "provider-model"},
		{"JSON without media type for compatible chat endpoints", llm.APIFormatOpenAIChatCompletion, `{"model":"provider-model"}`, "", "provider-model"},
		{"Anthropic", llm.APIFormatAnthropicMessage, `{"model":"claude-version"}`, "application/json", "claude-version"},
		{"Responses", llm.APIFormatOpenAIResponse, `{"model":"gpt-version"}`, "application/json", "gpt-version"},
		{"Compact", llm.APIFormatOpenAIResponseCompact, `{"model":"gpt-version"}`, "application/json", "gpt-version"},
		{"Gemini model version", llm.APIFormatGeminiContents, `{"modelVersion":"gemini-version","candidates":[]}`, "application/json", "gemini-version"},
		{"Antigravity", llm.APIFormatGeminiContents, `{"response":{"modelVersion":"gemini-version"}}`, "application/json", "gemini-version"},
		{"Cline", llm.APIFormatOpenAIChatCompletion, `{"success":true,"data":{"model":"provider-model"}}`, "application/json", "provider-model"},
		{"unrecognized envelope", llm.APIFormatOpenAIChatCompletion, `{"data":{"model":"generated"}}`, "application/json", ""},
		{"image without model", llm.APIFormatOpenAIImageGeneration, `{"created":123,"data":[{"b64_json":"image"}]}`, "application/json", ""},
		{"preserve spelling", llm.APIFormatOpenAIChatCompletion, `{"model":" Provider-Model "}`, "application/json", " Provider-Model "},
		{"generated content is not metadata", llm.APIFormatOpenAIChatCompletion, `{"choices":[{"message":{"content":{"model":"generated"}}}]}`, "application/json", ""},
		{"request echo is not metadata", llm.APIFormatOpenAIChatCompletion, `{"request":{"model":"requested"}}`, "application/json", ""},
		{"wrong protocol field", llm.APIFormatOpenAIChatCompletion, `{"modelVersion":"generated"}`, "application/json", ""},
		{"unsupported protocol", "custom/unknown", `{"model":"generated"}`, "application/json", ""},
		{"text transcript happens to be JSON", llm.APIFormatOpenAITranscription, `{"model":"whisper-1"}`, "text/plain", ""},
		{"text translation happens to be JSON", llm.APIFormatOpenAITranslation, `{"model":"whisper-1"}`, "text/plain; charset=utf-8", ""},
		{"ambiguous transcript media type", llm.APIFormatOpenAITranscription, `{"model":"whisper-1"}`, "", ""},
		{"JSON transcription metadata", llm.APIFormatOpenAITranscription, `{"model":"whisper-1","text":"hello"}`, "application/json; charset=utf-8", "whisper-1"},
		{"JSON transcript text remains content", llm.APIFormatOpenAITranscription, `{"text":"{\"model\":\"whisper-1\"}"}`, "application/json", ""},
		{"binary speech", llm.APIFormatOpenAISpeech, `{"model":"audio-data"}`, "audio/pcm", ""},
		{"speech is not model metadata even if marked JSON", llm.APIFormatOpenAISpeech, `{"model":"audio-data"}`, "application/json", ""},
		{"malformed media type falls back to the body", llm.APIFormatOpenAIChatCompletion, `{"model":"m"}`, "application/json; x=", "m"},
		{"structured JSON media type", llm.APIFormatOpenAIChatCompletion, `{"model":"m"}`, "application/vnd.provider+json", "m"},
		{"stream media type on a non-streaming chat response", llm.APIFormatOpenAIChatCompletion, `{"model":"provider-model","choices":[]}`, "text/event-stream", "provider-model"},
		{"stream media type on a non-streaming Anthropic response", llm.APIFormatAnthropicMessage, `{"model":"claude-version"}`, "text/event-stream", "claude-version"},
		{"stream media type on a non-streaming Responses response", llm.APIFormatOpenAIResponse, `{"model":"gpt-version"}`, "text/event-stream", "gpt-version"},
		{"text transcript keeps its content even when labelled SSE", llm.APIFormatOpenAITranscription, `{"model":"whisper-1"}`, "text/event-stream", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &httpclient.Response{StatusCode: http.StatusOK, Headers: http.Header{"Content-Type": {tt.contentType}}, Body: []byte(tt.body)}
			require.Equal(t, tt.want, ResponseModel(response, tt.format))
			response.StatusCode = http.StatusBadGateway
			require.Empty(t, ResponseModel(response, tt.format), "HTTP error metadata may echo the requested model")
		})
	}
	response := &httpclient.Response{Request: &httpclient.Request{APIFormat: string(llm.APIFormatGeminiContents)}, Body: []byte(`{"modelVersion":"gemini"}`)}
	require.Equal(t, "gemini", ResponseModel(response, llm.APIFormatOpenAIChatCompletion))
	require.Empty(t, ResponseModel(nil, llm.APIFormatOpenAIChatCompletion))
}

func TestStreamModel(t *testing.T) {
	tests := []struct {
		name                  string
		format                llm.APIFormat
		eventType, body, want string
	}{
		{"chat", llm.APIFormatOpenAIChatCompletion, "", `{"model":"m","choices":[]}`, "m"},
		{"Anthropic start", llm.APIFormatAnthropicMessage, "message_start", `{"message":{"model":"claude"}}`, "claude"},
		{"Anthropic start from data type", llm.APIFormatAnthropicMessage, "", `{"type":"message_start","message":{"model":"claude"}}`, "claude"},
		{"Anthropic content delta", llm.APIFormatAnthropicMessage, "content_block_delta", `{"message":{"model":"generated"},"model":"generated"}`, ""},
		{"Responses lifecycle", llm.APIFormatOpenAIResponse, "response.created", `{"response":{"model":"gpt"}}`, "gpt"},
		{"Responses failed terminal", llm.APIFormatOpenAIResponse, "response.failed", `{"response":{"model":"gpt","status":"failed"}}`, "gpt"},
		{"Responses output delta", llm.APIFormatOpenAIResponse, "response.output_text.delta", `{"model":"generated","response":{"model":"generated"}}`, ""},
		{"SSE event name wins over content type", llm.APIFormatOpenAIResponse, "response.output_text.delta", `{"type":"response.created","response":{"model":"generated"}}`, ""},
		{"Gemini", llm.APIFormatGeminiContents, "", `{"modelVersion":"gemini"}`, "gemini"},
		{"Ollama", llm.APIFormatOllamaChat, "", `{"model":"ollama"}`, "ollama"},
		{"audio bytes", llm.APIFormatOpenAIChatCompletion, "audio/pcm", `{"model":"audio-data"}`, ""},
		{"transcription text", llm.APIFormatOpenAITranscription, "", `{"model":"transcript"}`, ""},
		{"DONE", llm.APIFormatOpenAIChatCompletion, "", `[DONE]`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, StreamModel(&httpclient.StreamEvent{Type: tt.eventType, Data: []byte(tt.body)}, tt.format))
		})
	}
	require.Empty(t, StreamModel(nil, llm.APIFormatOpenAIChatCompletion))
}

func TestModelValidation(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"model":null}`, `{"model":123}`, `{"model":""}`, `{"model":"   "}`,
		`{"model":"bad\nname"}`, `{"model":"partial"`, `{"model":"` + strings.Repeat("a", maxModelBytes+1) + `"}`,
	} {
		require.Empty(t, ResponseModel(&httpclient.Response{Body: []byte(body)}, llm.APIFormatOpenAIChatCompletion))
	}
	require.Equal(t, strings.Repeat("a", maxModelBytes), ResponseModel(&httpclient.Response{Body: []byte(`{"model":"` + strings.Repeat("a", maxModelBytes) + `"}`)}, llm.APIFormatOpenAIChatCompletion))
	require.Equal(t, "模型-1", ResponseModel(&httpclient.Response{Body: []byte(`{"model":"模型-1"}`)}, llm.APIFormatOpenAIChatCompletion))
}
