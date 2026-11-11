package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

var errProtectedRewrite = errors.New("plugin rewrite changed protected fields")

type protocolControls struct {
	exact []string
	// defaults describes semantic defaults, not presence-sensitive equality.
	defaults        map[string]json.RawMessage
	endpointModel   bool
	endpointStream  bool
	embeddingModels bool
}

var (
	modelControls = protocolControls{exact: []string{"model"}}
	chatControls  = protocolControls{exact: []string{"model"}, defaults: map[string]json.RawMessage{"stream": json.RawMessage("false")}}
)

// Keep this table aligned with the outbound wire encoders in llm/transformer.
// Controls absent from a protocol are intentionally not reserved globally.
var attemptProtocolControls = map[llm.APIFormat]protocolControls{
	llm.APIFormatOpenAIChatCompletion: chatControls,
	llm.APIFormatOpenAICompletion:     chatControls,
	llm.APIFormatOpenAIResponse:       chatControls,
	llm.APIFormatAnthropicMessage:     chatControls,
	// ollama/outbound.go explicitly writes false because Ollama defaults to true.
	llm.APIFormatOllamaChat: {exact: []string{"model"}, defaults: map[string]json.RawMessage{"stream": json.RawMessage("true")}},
	// responses/protocol_codec.go and buildWebSocketCreatePayload normalize the frame.
	llm.APIFormatOpenAIResponseWebSocket: {exact: []string{"model"}, defaults: map[string]json.RawMessage{
		"stream": json.RawMessage("false"), "type": json.RawMessage(`"response.create"`), "generate": json.RawMessage("true"),
	}},
	// audio_outbound.go: absent stream_format selects binary audio, not SSE.
	llm.APIFormatOpenAISpeech: {exact: []string{"model"}, defaults: map[string]json.RawMessage{"stream_format": json.RawMessage(`"audio"`)}},
	// gemini/outbound.go encodes model and generateContent/streamGenerateContent in URL.
	llm.APIFormatGeminiContents: {endpointModel: true, endpointStream: true},
	// gemini/embedding.go repeats models/<id> inside batch request items.
	llm.APIFormatGeminiEmbedding:       {endpointModel: true, embeddingModels: true},
	llm.APIFormatOpenAIResponseCompact: modelControls,
	llm.APIFormatOpenAIImageGeneration: chatControls,
	llm.APIFormatOpenAIImageEdit:       chatControls,
	llm.APIFormatOpenAIImageVariation:  modelControls,
	llm.APIFormatOpenAIEmbedding:       modelControls,
	llm.APIFormatOpenAIVideo:           modelControls,
	llm.APIFormatOpenAITranscription:   chatControls,
	llm.APIFormatOpenAITranslation:     chatControls,
	llm.APIFormatOpenAIModeration:      modelControls,
	llm.APIFormatOpenAIAlphaSearch:     modelControls,
	llm.APIFormatOpenAIDecisions:       chatControls,
	llm.APIFormatJinaRerank:            modelControls,
	llm.APIFormatJinaEmbedding:         modelControls,
	llm.APIFormatTypeSafeSystemOne:     modelControls,
	llm.APIFormatSeedanceVideo:         modelControls,
	llm.APIFormatZenmuxVideo:           modelControls,
	llm.APIFormatModelScopeImage:       modelControls,
	// AI SDK formats are inbound-only today. Routing controls remain host-owned
	// if a future outbound uses these formats.
	llm.APIFormatAiSDKText:       chatControls,
	llm.APIFormatAiSDKDataStream: chatControls,
}

func protectedAttemptFields(request *httpclient.Request, after []byte, model string) []string {
	var a, b map[string]json.RawMessage
	if json.Unmarshal(request.Body, &a) != nil || json.Unmarshal(after, &b) != nil {
		return []string{"/"}
	}
	rule, ok := attemptProtocolControls[llm.APIFormat(request.APIFormat)]
	if !ok {
		return []string{"/"}
	} // Never silently invent controls for a new protocol.
	changed := map[string]bool{}
	for _, key := range rule.exact {
		if !sameJSONField(a, b, key) {
			changed[key] = true
		}
	}
	for key, fallback := range rule.defaults {
		av, aok := a[key]
		bv, bok := b[key]
		if !aok {
			av = fallback
		}
		if !bok {
			bv = fallback
		}
		if !jsonpatch.Equal(av, bv) {
			changed[key] = true
		}
	}
	if rule.endpointModel && !sameJSONField(a, b, "model") {
		// Deleting an explicit model or adding one that conflicts with the URL is forbidden.
		var value string
		if json.Unmarshal(b["model"], &value) != nil || strings.TrimPrefix(value, "models/") != strings.TrimPrefix(model, "models/") {
			changed["model"] = true
		}
	}
	if rule.endpointStream && !sameJSONField(a, b, "stream") {
		if value, exists := b["stream"]; exists {
			var stream bool
			if string(value) == "null" || json.Unmarshal(value, &stream) != nil || stream != strings.Contains(request.URL, ":streamGenerateContent") {
				changed["stream"] = true
			}
		}
	}
	if rule.embeddingModels && !sameJSONField(a, b, "requests") {
		var items []map[string]json.RawMessage
		if json.Unmarshal(b["requests"], &items) == nil {
			for _, item := range items {
				var value string
				if json.Unmarshal(item["model"], &value) != nil || strings.TrimPrefix(value, "models/") != strings.TrimPrefix(model, "models/") {
					changed["requests/*/model"] = true
					break
				}
			}
		}
	}
	fields := make([]string, 0, len(changed))
	for key := range changed {
		fields = append(fields, key)
	}
	sort.Strings(fields)
	return fields
}

func sameJSONField(a, b map[string]json.RawMessage, key string) bool {
	av, aok := a[key]
	bv, bok := b[key]
	return aok == bok && (!aok || jsonpatch.Equal(av, bv))
}

func (m *Manager) noteProtectedDrop(ctx context.Context, item *LoadedPlugin, hook, format string, fields []string) {
	m.hookMu.Lock()
	if m.hookCounters == nil {
		m.hookCounters = map[int]hookCounter{}
	}
	counter := m.hookCounters[item.ID]
	counter.protectedDrops++
	m.hookCounters[item.ID] = counter
	m.hookMu.Unlock()
	requestID, _ := contexts.GetRequestID(ctx)
	log.Warn(ctx, "plugin rewrite discarded",
		log.String("plugin_id", item.PluginID), log.String("hook", hook), log.String("request_id", requestID),
		log.String("api_format", format), log.Strings("changed_fields", fields), log.String("reason", "protected_fields_changed"),
	)
}

// protectedRequestFields preserves the transport selected by the API handler.
func protectedRequestFields(before, after *llm.Request) []string {
	var fields []string
	if !sameStreamShape(before.Stream, after.Stream) {
		fields = append(fields, "stream")
	}
	if before.Speech != nil {
		// Missing stream_format uses audio on the wire, but also selects a different
		// host streaming path. Preserve the inbound setting exactly.
		if after.Speech == nil || before.Speech.StreamFormat != after.Speech.StreamFormat {
			fields = append(fields, "speech.stream_format")
		}
	}
	return fields
}
