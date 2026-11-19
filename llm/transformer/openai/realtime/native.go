// 职责: 保真代理 OpenAI Realtime GA 事件并只读解析安全与计量元数据。
// 关联文件:
//
//	llm/model.go
//	internal/server/orchestrator/realtime.go
//
// 注意事项:
//
//	不重写合法事件、未知扩展字段或音频；禁止会话更换模型及未经批准的远程 MCP。
package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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

type Adapter struct{ Capability Capability }

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
	if err != nil || u.Scheme != "wss" || u.Host != "api.openai.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/" && u.Path != "/v1/realtime") {
		return errors.New("OpenAI Realtime requires the trusted wss://api.openai.com host")
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
	return "wss://api.openai.com/v1/realtime?" + url.Values{"model": []string{model}}.Encode(), nil
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
				return nil, errors.New("session.type must be realtime")
			}
		}
		for _, f := range []string{"model", "authorization", "api_key", "safety_identifier"} {
			if _, exists := body[f]; exists {
				return nil, errors.New("security control fields cannot be overridden")
			}
		}
		// Independent input transcription has a separate usage stream and rate card.
		// Do not enable that billable feature until its own metering is installed.
		if field == "session" {
			if raw, exists := body["input_audio_transcription"]; exists && string(raw) != "null" {
				return nil, errors.New("independent transcription is not enabled")
			}
			var audio struct {
				Input struct {
					Transcription json.RawMessage `json:"transcription"`
				} `json:"input"`
			}
			if raw, exists := body["audio"]; exists && json.Unmarshal(raw, &audio) == nil {
				if len(audio.Input.Transcription) > 0 && string(audio.Input.Transcription) != "null" {
					return nil, errors.New("independent input transcription is not enabled without separate usage accounting")
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
	if event.Type == "error" {
		// Authentication details and provider validation echoes can include secrets or media.
		var envelope map[string]json.RawMessage
		_ = json.Unmarshal(data, &envelope)
		envelope["error"] = json.RawMessage(`{"type":"invalid_request_error","code":"upstream_error","message":"The upstream Realtime service rejected an event."}`)
		result, err := json.Marshal(envelope)
		return result, err
	}
	if event.Type == "response.done" {
		var envelope map[string]json.RawMessage
		_ = json.Unmarshal(data, &envelope)
		var response map[string]json.RawMessage
		_ = json.Unmarshal(envelope["response"], &response)
		var details map[string]json.RawMessage
		_ = json.Unmarshal(response["status_details"], &details)
		if _, exists := details["error"]; exists {
			details["error"] = json.RawMessage(`{"type":"server_error","code":"upstream_error","message":"The upstream Realtime response failed."}`)
			response["status_details"], _ = json.Marshal(details)
			envelope["response"], _ = json.Marshal(response)
			return json.Marshal(envelope)
		}
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
