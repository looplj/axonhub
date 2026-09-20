// Package modelmetadata reads model identifiers from protocol metadata, never
// from synthesized llm.Response.Model values or arbitrary generated content.
package modelmetadata

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"slices"
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

func mediaType(value string) string {
	if value == "" {
		return ""
	}
	typ, _, err := mime.ParseMediaType(value)
	if err != nil {
		return "invalid"
	}
	return strings.ToLower(typ)
}

func isJSON(typ string) bool {
	return typ == "application/json" || strings.HasSuffix(typ, "+json")
}

// SentModel reads the final wire request, not JSONBody (a logging summary),
// TransformerMetadata, or the model chosen before channel overrides.
func SentModel(request *httpclient.Request, format llm.APIFormat) string {
	if request == nil {
		return ""
	}
	if request.APIFormat != "" {
		format = llm.APIFormat(request.APIFormat)
	}
	contentType := request.ContentType
	if contentType == "" {
		contentType = request.Headers.Get("Content-Type")
	}
	typ := mediaType(contentType)
	if typ == "multipart/form-data" {
		switch format {
		case llm.APIFormatOpenAIImageEdit, llm.APIFormatOpenAIImageVariation,
			llm.APIFormatOpenAITranscription, llm.APIFormatOpenAITranslation, llm.APIFormatOpenAIVideo:
		default:
			return ""
		}
		_, params, _ := mime.ParseMediaType(contentType)
		reader := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"])
		var model string
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				return model
			}
			if err != nil {
				return ""
			}
			if part.FormName() != "model" || part.FileName() != "" {
				continue
			}
			value, err := io.ReadAll(io.LimitReader(part, maxModelBytes+1))
			if err != nil || !validModel(string(value)) || (model != "" && model != string(value)) {
				return ""
			}
			model = string(value)
		}
	}
	if typ != "" && !isJSON(typ) {
		return ""
	}
	// Gemini identifies the model in the URL, except for Antigravity's
	// envelope, which carries the transformed model at its top level.
	if format == llm.APIFormatGeminiContents || format == llm.APIFormatGeminiEmbedding {
		if _, ok := request.Metadata["antigravity_model"]; ok {
			return jsonModel(request.Body, "model")
		}
		return modelInURL(request.URL, "/models/", ":generateContent", ":streamGenerateContent", ":embedContent", ":batchEmbedContents", ":predict")
	}
	if format == llm.APIFormatAnthropicMessage {
		if model := modelInURL(request.URL, "/model/", "/invoke", "/invoke-with-response-stream", "/converse", "/converse-stream"); model != "" {
			return model
		}
	}
	switch format {
	case llm.APIFormatOpenAIChatCompletion, llm.APIFormatOpenAICompletion,
		llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit, llm.APIFormatOpenAIImageVariation,
		llm.APIFormatOpenAIEmbedding, llm.APIFormatOpenAIModeration, llm.APIFormatOpenAIVideo,
		llm.APIFormatOpenAISpeech, llm.APIFormatOpenAITranscription, llm.APIFormatOpenAITranslation,
		llm.APIFormatAnthropicMessage, llm.APIFormatOllamaChat, llm.APIFormatJinaEmbedding, llm.APIFormatJinaRerank,
		llm.APIFormatSeedanceVideo, llm.APIFormatZenmuxVideo:
		return jsonModel(request.Body, "model")
	default:
		return ""
	}
}

func modelInURL(rawURL, marker string, suffixes ...string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	_, model, found := strings.Cut(u.EscapedPath(), marker)
	if !found {
		return ""
	}
	for _, suffix := range suffixes {
		if remaining, found := strings.CutSuffix(model, suffix); found {
			model, err = url.PathUnescape(remaining)
			if err == nil && validModel(model) {
				return model
			}
		}
	}
	return ""
}

// ResponseModel respects the declared media type. A text transcript that happens
// to be valid JSON is still generated text, not response metadata.
func ResponseModel(response *httpclient.Response, format llm.APIFormat) string {
	if response == nil || response.StatusCode >= 400 {
		return ""
	}
	if response.Request != nil && response.Request.APIFormat != "" {
		format = llm.APIFormat(response.Request.APIFormat)
	}
	typ := mediaType(response.Headers.Get("Content-Type"))
	if typ != "" && !isJSON(typ) {
		return ""
	}
	if format == llm.APIFormatOpenAISpeech {
		return ""
	}
	if (format == llm.APIFormatOpenAITranscription || format == llm.APIFormatOpenAITranslation) && !isJSON(typ) {
		return ""
	}
	return reportedModel(response.Body, format, "", false)
}

func StreamModel(event *httpclient.StreamEvent, format llm.APIFormat) string {
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
	case llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact:
		if !stream {
			return jsonModel(body, "model")
		}
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
		llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit, llm.APIFormatOpenAIImageVariation,
		llm.APIFormatOpenAITranscription, llm.APIFormatOpenAITranslation,
		llm.APIFormatOpenAIVideo, llm.APIFormatJinaEmbedding, llm.APIFormatJinaRerank,
		llm.APIFormatSeedanceVideo, llm.APIFormatZenmuxVideo:
		if !stream {
			return jsonModel(body, "model")
		}
	}
	return ""
}

// Observe retains the first identifier and the first different identifier.
// Two values are enough to prove a conflict without unbounded per-stream state.
func Observe(models []string, model string) []string {
	if len(models) >= 2 || !validModel(model) || slices.Contains(models, model) {
		return models
	}
	return append(models, model)
}
