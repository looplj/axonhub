package modelname

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestFromResponse(t *testing.T) {
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
		{"TypeSafe", llm.APIFormatTypeSafeSystemOne, `{"model":"jev-1.13.0","answers":{}}`, "application/json", "jev-1.13.0"},
		{"TypeSafe answers are not metadata", llm.APIFormatTypeSafeSystemOne, `{"answers":{"model":"generated"}}`, "application/json", ""},
		{"Alpha Search results are not metadata", llm.APIFormatOpenAIAlphaSearch, `{"results":[{"model":"search-result"}]}`, "application/json", ""},
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
			require.Equal(t, tt.want, FromResponse(response, tt.format))
			response.StatusCode = http.StatusBadGateway
			require.Empty(t, FromResponse(response, tt.format), "HTTP error metadata may echo the requested model")
		})
	}
	response := &httpclient.Response{Request: &httpclient.Request{APIFormat: string(llm.APIFormatGeminiContents)}, Body: []byte(`{"modelVersion":"gemini"}`)}
	require.Equal(t, "gemini", FromResponse(response, llm.APIFormatOpenAIChatCompletion))
	require.Empty(t, FromResponse(nil, llm.APIFormatOpenAIChatCompletion))
}

func TestFromEvent(t *testing.T) {
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
		{"Codex image generation", llm.APIFormatOpenAIImageGeneration, "response.created", `{"response":{"model":"gpt-5.4-mini"}}`, "gpt-5.4-mini"},
		{"Codex image edit terminal", llm.APIFormatOpenAIImageEdit, "", `{"type":"response.completed","response":{"model":"gpt-5.4-mini"}}`, "gpt-5.4-mini"},
		{"Codex image output delta is not metadata", llm.APIFormatOpenAIImageGeneration, "response.output_text.delta", `{"response":{"model":"generated"}}`, ""},
		{"Codex image SSE name wins", llm.APIFormatOpenAIImageEdit, "response.output_text.delta", `{"type":"response.created","response":{"model":"generated"}}`, ""},
		{"native image stream without model metadata", llm.APIFormatOpenAIImageGeneration, "image_generation.completed", `{"b64_json":"image"}`, ""},
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
			require.Equal(t, tt.want, FromEvent(&httpclient.StreamEvent{Type: tt.eventType, Data: []byte(tt.body)}, tt.format))
		})
	}
	require.Empty(t, FromEvent(nil, llm.APIFormatOpenAIChatCompletion))
}

func TestModelValidation(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"model":null}`, `{"model":123}`, `{"model":""}`, `{"model":"   "}`,
		`{"model":"bad\nname"}`, `{"model":"partial"`, `{"model":"` + strings.Repeat("a", maxModelBytes+1) + `"}`,
	} {
		require.Empty(t, FromResponse(&httpclient.Response{Body: []byte(body)}, llm.APIFormatOpenAIChatCompletion))
	}
	require.Equal(t, strings.Repeat("a", maxModelBytes), FromResponse(&httpclient.Response{Body: []byte(`{"model":"` + strings.Repeat("a", maxModelBytes) + `"}`)}, llm.APIFormatOpenAIChatCompletion))
	require.Equal(t, "模型-1", FromResponse(&httpclient.Response{Body: []byte(`{"model":"模型-1"}`)}, llm.APIFormatOpenAIChatCompletion))
}
