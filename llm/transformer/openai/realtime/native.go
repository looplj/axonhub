// 职责: 保真代理 OpenAI Realtime GA 事件并只读解析安全与计量元数据。
// 关联文件:
//
//	llm/model.go
//	internal/server/orchestrator/realtime.go
//
// 注意事项:
//
//	保留合法扩展字段与音频。转发会话更新前移除已核对的模型字段与未计量的转录设置。
package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/llm"
)

type Capability struct {
	Model                       string
	ImageInput, FunctionCalling bool
	PCMRate                     int
}

var capabilities = map[string]Capability{
	"gpt-realtime-2.1":      {Model: "gpt-realtime-2.1", ImageInput: true, FunctionCalling: true, PCMRate: 24000},
	"gpt-realtime-2.1-mini": {Model: "gpt-realtime-2.1-mini", ImageInput: true, FunctionCalling: true, PCMRate: 24000},
}

var (
	ErrModelSwitch         = errors.New("session.model must match the model selected at connection time")
	ErrSessionType         = errors.New("session.type must be realtime")
	ErrLegacySessionFields = errors.New("use session.output_modalities and nested session.audio input/output settings instead of flat session fields")
)

type Adapter struct {
	Capability  Capability
	ClientModel string
}

func NewAdapter(model string) (*Adapter, error) {
	c, ok := capabilities[model]
	if !ok {
		return nil, errors.New("unsupported OpenAI Realtime model")
	}
	return &Adapter{Capability: c}, nil
}
func ValidateEndpoint(baseURL, path string) error {
	if baseURL == "" {
		baseURL = "wss://api.openai.com"
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "wss" || u.Hostname() == "" || net.ParseIP(u.Hostname()) != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("OpenAI Realtime requires a wss:// host without a path or query")
	}
	if path != "" && path != "/v1/realtime" {
		return errors.New("OpenAI Realtime path must be /v1/realtime")
	}
	return nil
}
func EndpointURL(baseURL, path, model string) (string, error) {
	if err := ValidateEndpoint(baseURL, path); err != nil {
		return "", err
	}
	if _, err := NewAdapter(model); err != nil {
		return "", err
	}
	if baseURL == "" {
		baseURL = "wss://api.openai.com"
	}
	return strings.TrimSuffix(baseURL, "/") + "/v1/realtime?" + url.Values{"model": []string{model}}.Encode(), nil
}
func (a *Adapter) Client(data []byte) ([]byte, error) {
	var event map[string]json.RawMessage
	if len(data) > 256<<10 || json.Unmarshal(data, &event) != nil {
		return nil, errors.New("invalid realtime event")
	}
	var kind string
	if json.Unmarshal(event["type"], &kind) != nil || kind == "" {
		return nil, errors.New("event type is required")
	}
	if strings.HasPrefix(kind, "transcription_session.") {
		return nil, errors.New("separate transcription sessions are not enabled")
	}
	for _, field := range []string{"model", "authorization", "api_key", "safety_identifier"} {
		if _, exists := event[field]; exists {
			return nil, errors.New("security control fields cannot be overridden")
		}
	}
	if kind == "session.update" || kind == "response.create" {
		field := "response"
		if kind == "session.update" {
			field = "session"
		}
		var body map[string]json.RawMessage
		if len(event[field]) > 0 && json.Unmarshal(event[field], &body) != nil {
			return nil, errors.New("invalid realtime configuration")
		}
		if kind == "session.update" {
			var sessionType string
			if body == nil || json.Unmarshal(body["type"], &sessionType) != nil || sessionType != "realtime" {
				return nil, ErrSessionType
			}
		}
		for _, f := range []string{"authorization", "api_key", "safety_identifier"} {
			if _, exists := body[f]; exists {
				return nil, errors.New("security control fields cannot be overridden")
			}
		}
		changed := false
		if raw, exists := body["model"]; exists {
			selected := a.ClientModel
			if selected == "" {
				selected = a.Capability.Model
			}
			var model string
			if kind != "session.update" || json.Unmarshal(raw, &model) != nil || model != selected {
				return nil, ErrModelSwitch
			}
			delete(body, "model")
			changed = true
		}
		if kind == "session.update" {
			for _, legacy := range []string{"modalities", "turn_detection", "input_audio_format", "output_audio_format"} {
				if _, exists := body[legacy]; exists {
					return nil, ErrLegacySessionFields
				}
			}
			// Independent transcription is not metered. Remove SDK transcription
			// settings before forwarding so they cannot activate an upstream charge.
			if _, exists := body["input_audio_transcription"]; exists {
				delete(body, "input_audio_transcription")
				changed = true
			}
			var audio, input map[string]json.RawMessage
			if json.Unmarshal(body["audio"], &audio) == nil && json.Unmarshal(audio["input"], &input) == nil {
				if raw, exists := input["transcription"]; exists && string(raw) != "null" {
					delete(input, "transcription")
					if len(input) == 0 {
						delete(audio, "input")
					} else {
						audio["input"], _ = json.Marshal(input)
					}
					if len(audio) == 0 {
						delete(body, "audio")
					} else {
						body["audio"], _ = json.Marshal(audio)
					}
					changed = true
				}
			}
		}
		var tools []struct {
			Type string `json:"type"`
		}
		if raw, ok := body["tools"]; ok {
			if json.Unmarshal(raw, &tools) != nil {
				return nil, errors.New("invalid tools")
			}
			for _, tool := range tools {
				if tool.Type != "function" {
					return nil, errors.New("only function tools are enabled; remote or billable tools require an explicit gateway policy")
				}
			}
		}
		if changed {
			event[field], _ = json.Marshal(body)
			return json.Marshal(event)
		}
	}
	if kind == "input_audio_buffer.append" {
		var audio string
		if json.Unmarshal(event["audio"], &audio) != nil || audio == "" {
			return nil, errors.New("audio is required")
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(audio)
		if err != nil || len(decoded) > 64<<10 {
			return nil, errors.New("audio block must be valid Base64 and at most 64 KiB")
		}
	}
	return data, nil
}
func (a *Adapter) Server(data []byte) ([]byte, error) {
	var event struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(data, &event) != nil || event.Type == "" {
		return nil, errors.New("invalid upstream realtime event")
	}

	return data, nil
}

type tokenDetails struct {
	Text          *int64        `json:"text_tokens"`
	Audio         *int64        `json:"audio_tokens"`
	Image         *int64        `json:"image_tokens"`
	Cached        *int64        `json:"cached_tokens"`
	CachedDetails *tokenDetails `json:"cached_tokens_details"`
}

func value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
func (t *tokenDetails) valid() bool {
	return t != nil && value(t.Text) >= 0 && value(t.Audio) >= 0 && value(t.Image) >= 0 && value(t.Cached) >= 0
}
func ParseUsage(data []byte) (string, *llm.Usage, error) {
	var event struct {
		Type     string `json:"type"`
		Response struct {
			ID    string `json:"id"`
			Usage *struct {
				Input  *int64        `json:"input_tokens"`
				Output *int64        `json:"output_tokens"`
				Total  *int64        `json:"total_tokens"`
				In     *tokenDetails `json:"input_token_details"`
				Out    *tokenDetails `json:"output_token_details"`
			} `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return "", nil, errors.New("invalid realtime usage event")
	}
	if event.Type != "response.done" {
		return "", nil, nil
	}
	id, u := event.Response.ID, event.Response.Usage
	if id == "" || u == nil || u.Input == nil || u.Output == nil || u.Total == nil || *u.Input < 0 || *u.Output < 0 || *u.Total < 0 || !u.In.valid() || !u.Out.valid() {
		return id, nil, errors.New("realtime usage is unknown or invalid")
	}
	// Subtractions avoid integer-overflow acceptance of forged totals.
	if *u.Input > *u.Total || *u.Output != *u.Total-*u.Input || value(u.In.Text) > *u.Input || value(u.In.Audio) > *u.Input-value(u.In.Text) || value(u.In.Image) != *u.Input-value(u.In.Text)-value(u.In.Audio) || value(u.Out.Text) > *u.Output || value(u.Out.Audio) != *u.Output-value(u.Out.Text) || value(u.Out.Image) != 0 {
		return id, nil, errors.New("realtime modality totals do not reconcile")
	}
	cached := u.In.CachedDetails
	if cached == nil {
		if value(u.In.Cached) != 0 {
			return id, nil, errors.New("cached usage breakdown is unknown")
		}
		cached = &tokenDetails{}
	}
	if !cached.valid() || value(cached.Text) > value(u.In.Text) || value(cached.Audio) > value(u.In.Audio) || value(cached.Image) > value(u.In.Image) || value(cached.Text)+value(cached.Audio)+value(cached.Image) != value(u.In.Cached) {
		return id, nil, errors.New("cached realtime usage does not reconcile")
	}
	usage := &llm.Usage{PromptTokens: *u.Input, CompletionTokens: *u.Output, TotalTokens: *u.Total, PromptTokensDetails: &llm.PromptTokensDetails{AudioTokens: value(u.In.Audio), CachedTokens: value(u.In.Cached), ImageTokens: value(u.In.Image)}, CompletionTokensDetails: &llm.CompletionTokensDetails{AudioTokens: value(u.Out.Audio)}, Realtime: &llm.RealtimeUsageDetails{InputTextTokens: value(u.In.Text), InputAudioTokens: value(u.In.Audio), InputImageTokens: value(u.In.Image), CachedInputTextTokens: value(cached.Text), CachedInputAudioTokens: value(cached.Audio), CachedInputImageTokens: value(cached.Image), OutputTextTokens: value(u.Out.Text), OutputAudioTokens: value(u.Out.Audio)}}
	return strings.TrimSpace(id), usage, nil
}
