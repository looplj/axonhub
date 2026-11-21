// 职责: 校验 OpenAI 实时翻译的独立端点与会话事件。
// 关联文件:
//   llm/transformer/openai/realtime/native.go
//   internal/server/orchestrator/realtime_translation.go
// 注意事项: 上游不报告账单用量，音频时长来自发出的 PCM 样本。

package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	TranslationModel            = "gpt-realtime-translate"
	TranslationSamplesPerMinute = 24000 * 60
)

func TranslationEndpointURL(baseURL, path, model string) (string, error) {
	if err := ValidateEndpoint(baseURL, path); err != nil {
		return "", err
	}
	if model != TranslationModel {
		return "", errors.New("unsupported OpenAI Realtime Translation model")
	}
	return "wss://api.openai.com/v1/realtime/translations?" + url.Values{"model": []string{model}}.Encode(), nil
}

// Translation serializes state through the owning conversation.
type Translation struct {
	created, ready, closing, closed bool
	pendingID                       string
	pendingAt                       time.Time
}

func (t *Translation) Expired() bool {
	return t.pendingID != "" && time.Since(t.pendingAt) > 15*time.Second
}

func (t *Translation) Client(data []byte) ([]byte, error) {
	if !t.created || t.closing || t.closed {
		return nil, errors.New("invalid event or translation session is closing")
	}
	// Meter and forward the same type and audio even if the client supplies
	// duplicate envelope keys. JSON decoding resolves those keys before routing.
	var event map[string]json.RawMessage
	if err := json.Unmarshal(data, &event); err != nil || event == nil {
		return nil, errors.New("invalid translation client event")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	switch gjson.GetBytes(data, "type").String() {
	case "session.update":
		session := gjson.GetBytes(data, "session")
		if !session.IsObject() || session.Get("model").Exists() || session.Get("type").Exists() || t.pendingID != "" {
			return nil, errors.New("invalid translation session update or update pending")
		}
		id := gjson.GetBytes(data, "event_id")
		if id.Exists() && (id.Type != gjson.String || len(id.String()) > 256) {
			return nil, errors.New("invalid translation event ID")
		}
		eventID := id.String()
		if eventID == "" {
			eventID = "evt_axonhub_" + uuid.NewString()
			wire, err := sjson.SetBytes(data, "event_id", eventID)
			if err != nil {
				return nil, err
			}
			data = wire
		}
		t.pendingID, t.pendingAt = eventID, time.Now()
	case "session.input_audio_buffer.append":
		if !t.ready || t.pendingID != "" {
			return nil, errors.New("translation output language must be confirmed before appending audio")
		}
		if _, err := TranslationAudioSamples(data); err != nil {
			return nil, err
		}
	case "session.close":
		t.closing = true
	default:
		return nil, errors.New("unsupported translation client event")
	}
	return data, nil
}

func (t *Translation) Server(data []byte) ([]byte, error) {
	if !gjson.ValidBytes(data) || t.closed {
		return nil, errors.New("invalid translation server event")
	}
	switch gjson.GetBytes(data, "type").String() {
	case "session.created", "session.updated":
		session := gjson.GetBytes(data, "session")
		created := gjson.GetBytes(data, "type").String() == "session.created"
		if session.Get("type").String() != "translation" || session.Get("model").String() != TranslationModel ||
			(created && t.created) || (!created && t.pendingID == "") {
			return nil, errors.New("unexpected translation session configuration")
		}
		t.created = true
		t.ready = session.Get("audio.output.language").String() != ""
		t.pendingID = ""
	case "error":
		if t.pendingID != "" && gjson.GetBytes(data, "error.event_id").String() == t.pendingID {
			t.pendingID = ""
		}
	case "session.closed":
		t.closed = true
	case "session.output_audio.delta", "session.output_transcript.delta", "session.input_transcript.delta":
		// The original frame carries the provider's audio or transcript delta.
	default:
		return nil, errors.New("unsupported translation server event")
	}
	return data, nil
}

func TranslationAudioSamples(data []byte) (int64, error) {
	audio := gjson.GetBytes(data, "audio")
	if audio.Type != gjson.String || audio.String() == "" {
		return 0, errors.New("translation audio must contain base64 PCM16 samples")
	}
	pcm, err := base64.StdEncoding.Strict().DecodeString(audio.String())
	if err != nil || len(pcm) == 0 || len(pcm)%2 != 0 {
		return 0, errors.New("invalid translation PCM16 audio")
	}
	return int64(len(pcm) / 2), nil
}
