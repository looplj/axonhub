// Package modelname reads provider-reported model names from protocol metadata, never
// from synthesized llm.Response.Model values or arbitrary generated content.
package modelname

import (
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

const maxModelBytes = 512

func validModel(value string) bool {
	return len(value) <= maxModelBytes && utf8.ValidString(value) && strings.TrimSpace(value) != "" &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

func jsonModel(body []byte, paths ...string) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if value.Type == gjson.String && validModel(value.String()) {
			return value.String()
		}
	}
	return ""
}

// FromResponse reads the model a non-streaming provider response reports.
// Chat-style protocols carry the name as top-level metadata, so the media type is
// not gated: relays that label a JSON body as text/event-stream (or send a
// malformed one) would otherwise lose a name that is conclusively present. The
// body still has to parse as JSON and hold the name on a known metadata path.
// Text-bearing endpoints keep the gate, because their body is generated content
// that may coincidentally be valid JSON.
func FromResponse(response *httpclient.Response, format llm.APIFormat) string {
	if response == nil || response.StatusCode >= 400 {
		return ""
	}
	if response.Request != nil && response.Request.APIFormat != "" {
		format = llm.APIFormat(response.Request.APIFormat)
	}
	if format == llm.APIFormatOpenAITranscription || format == llm.APIFormatOpenAITranslation {
		typ, _, err := mime.ParseMediaType(response.Headers.Get("Content-Type"))
		if err != nil || (typ != "application/json" && !strings.HasSuffix(typ, "+json")) {
			return ""
		}
	}
	return reportedModel(response.Body, format, "", false)
}

// FromEvent reads the model name reported in a provider stream event.
func FromEvent(event *httpclient.StreamEvent, format llm.APIFormat) string {
	if event == nil || event.IsBinaryAudioChunk() {
		return ""
	}
	return reportedModel(event.Data, format, event.Type, true)
}

func reportedModel(body []byte, format llm.APIFormat, eventType string, stream bool) string {
	if eventType == "" {
		eventType = gjson.GetBytes(body, "type").String()
	}
	switch format {
	case llm.APIFormatAnthropicMessage:
		if !stream {
			return jsonModel(body, "model")
		}
		if eventType == "message_start" {
			return jsonModel(body, "message.model")
		}
	case llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit:
		if !stream {
			return jsonModel(body, "model")
		}
		// Codex preserves the image API format on requests while returning
		// Responses SSE. Only lifecycle metadata identifies the reported model.
		switch eventType {
		case "response.created", "response.in_progress", "response.completed", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
			return jsonModel(body, "response.model")
		}
	case llm.APIFormatGeminiContents, llm.APIFormatGeminiEmbedding:
		return jsonModel(body, "modelVersion", "response.modelVersion")
	case llm.APIFormatOpenAIChatCompletion, llm.APIFormatOpenAICompletion, llm.APIFormatOllamaChat:
		// Cline wraps an OpenAI-compatible response in a success/data envelope.
		if gjson.GetBytes(body, "success").Type == gjson.True {
			return jsonModel(body, "data.model")
		}
		return jsonModel(body, "model")
	case llm.APIFormatOpenAIEmbedding, llm.APIFormatOpenAIModeration,
		llm.APIFormatOpenAIImageVariation, llm.APIFormatTypeSafeSystemOne,
		llm.APIFormatOpenAITranscription, llm.APIFormatOpenAITranslation,
		llm.APIFormatOpenAIVideo, llm.APIFormatJinaEmbedding, llm.APIFormatJinaRerank,
		llm.APIFormatSeedanceVideo, llm.APIFormatZenmuxVideo:
		if !stream {
			return jsonModel(body, "model")
		}
	}
	return ""
}
