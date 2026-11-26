// 职责: 通过独立事件结构将 OpenAI GA 与 Qwen 实时协议互转并校验音频契约。
// 关联文件:
//
//	llm/realtime/capability.go
//	llm/realtime/resample.go
//	llm/transformer/openai/realtime/native.go
//
// 注意事项:
//
//	GA 音频为单声道 PCM16 24 kHz；Qwen3.8 真实降采样到 16 kHz，3.5 协商 24 kHz。
//	未支持的控制字段拒绝，不静默忽略；原生入口同样拒绝远程 MCP、视频和多声道。
package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sync"

	"github.com/looplj/axonhub/llm"
	nativega "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

type Event struct {
	Type   string
	ID     string
	Fields map[string]any
}

func decode(data []byte) (Event, error) {
	var fields map[string]any
	err := json.Unmarshal(data, &fields)
	kind, _ := fields["type"].(string)
	id, _ := fields["event_id"].(string)
	if err != nil || kind == "" || len(data) > 256<<10 {
		return Event{}, errors.New("invalid realtime event")
	}
	return Event{Type: kind, ID: id, Fields: fields}, nil
}

func object(v any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("expected a realtime object")
	}
	return m, nil
}

var ErrUnsupportedField = errors.New("unsupported realtime field")

// UnsupportedFieldError carries only the rejected JSON key and its schema path.
type UnsupportedFieldError struct{ cause error }

func (e *UnsupportedFieldError) Error() string {
	return e.cause.Error()
}

func (e *UnsupportedFieldError) Unwrap() error { return e.cause }

func only(m map[string]any, names ...string) error {
	return onlyAt("", m, names...)
}

func onlyAt(prefix string, m map[string]any, names ...string) error {
	allowed := map[string]bool{}
	for _, n := range names {
		allowed[n] = true
	}
	for key := range m {
		if !allowed[key] {
			// Bound the name and avoid reflecting arbitrary free-form payloads.
			if len(key) == 0 || len(key) > 64 {
				return ErrUnsupportedField
			}
			for _, r := range key {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
					return ErrUnsupportedField
				}
			}
			return &UnsupportedFieldError{cause: fmt.Errorf("%w %q", ErrUnsupportedField, prefix+key)}
		}
	}
	return nil
}
func number(v any) (int, bool) { n, ok := v.(float64); return int(n), ok && n == float64(int(n)) }

type Adapter struct {
	model                           string
	native                          bool
	cap                             capability
	mu                              sync.Mutex
	resampler                       *PCM24To16
	mediaStarted                    bool
	inputRate, outputRate           int
	confirmedInput, confirmedOutput int
	pendingUpdateID                 string
}

func NewAdapter(model string, native bool) (*Adapter, error) {
	c, ok := capabilityIndex[model]
	if !ok {
		return nil, errUnsupportedModel(model)
	}
	a := &Adapter{model: model, native: native, cap: c, inputRate: NativeInputRate(model), outputRate: 24000, confirmedInput: NativeInputRate(model), confirmedOutput: 24000}
	if !native && NativeInputRate(model) == 16000 {
		a.resampler = NewPCM24To16()
	}
	return a, nil
}

func (a *Adapter) Client(data []byte) (result []byte, clientErr error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	oldInput, oldOutput := a.inputRate, a.outputRate
	defer func() {
		if clientErr != nil {
			a.inputRate = oldInput
			a.outputRate = oldOutput
		}
	}()
	event, err := decode(data)
	if err != nil {
		return nil, err
	}
	e := event.Fields
	switch event.Type {
	case "session.update":
		if err := only(e, "type", "event_id", "session"); err != nil {
			return nil, err
		}
		s, err := object(e["session"])
		if err != nil {
			return nil, errors.New("session.update requires session")
		}
		if a.native {
			err = a.nativeSession(s)
		} else {
			err = a.gaSession(s)
		}
		if err != nil {
			return nil, err
		}
	case "input_audio_buffer.append":
		if err := only(e, "type", "event_id", "audio"); err != nil {
			return nil, err
		}
		audio, ok := e["audio"].(string)
		if !ok || audio == "" {
			return nil, errors.New("audio is required")
		}
		pcm, err := base64.StdEncoding.Strict().DecodeString(audio)
		if err != nil || len(pcm) > 64<<10 || len(pcm)%2 != 0 {
			return nil, errors.New("audio must be Base64 PCM16 complete frames up to 64 KiB")
		}
		a.mediaStarted = true
		if a.resampler != nil {
			pcm, err = a.resampler.Append(pcm)
			if err != nil {
				return nil, err
			}
			if len(pcm) == 0 {
				return nil, nil
			}
			e["audio"] = base64.StdEncoding.EncodeToString(pcm)
		}
	case "input_audio_buffer.commit", "input_audio_buffer.clear", "response.cancel":
		if err := only(e, "type", "event_id", "response_id"); err != nil {
			return nil, err
		}
		if event.Type == "input_audio_buffer.clear" && a.resampler != nil {
			a.resampler = NewPCM24To16()
		}
	case "response.create":
		if err := only(e, "type", "event_id", "response"); err != nil {
			return nil, err
		}
		if response, exists := e["response"]; exists {
			r, err := object(response)
			if err != nil || len(r) != 0 {
				return nil, errors.New("per-response overrides are not supported by the Qwen adapter")
			}
			delete(e, "response")
		}
	case "conversation.item.create":
		if err := only(e, "type", "event_id", "item", "previous_item_id"); err != nil {
			return nil, err
		}
		item, err := object(e["item"])
		if err != nil {
			return nil, err
		}
		if err := validateItem(item, !a.native); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported Qwen realtime client event")
	}
	if event.Type == "session.update" && !a.cap.FreeAudioTranscript {
		session, _ := object(e["session"])
		flattenFixedPCM(session)
	}
	wire, err := json.Marshal(e)
	if err == nil && event.Type == "session.update" {
		a.pendingUpdateID, _ = e["event_id"].(string)
	}
	return wire, err
}

func validateItem(item map[string]any, ga bool) error {
	if err := only(item, "id", "type", "role", "content", "call_id", "output"); err != nil {
		return err
	}
	switch item["type"] {
	case "function_call_output":
		callID, ok := item["call_id"].(string)
		if !ok || callID == "" {
			return errors.New("function result requires call_id")
		}
		if _, ok := item["output"].(string); !ok {
			return errors.New("function result requires string output")
		}
	case "message":
		if item["role"] != "user" && item["role"] != "assistant" {
			return errors.New("unsupported item role")
		}
		parts, ok := item["content"].([]any)
		if !ok || len(parts) == 0 {
			return errors.New("text content required")
		}
		for _, part := range parts {
			p, err := object(part)
			if err != nil {
				return err
			}
			if err := only(p, "type", "text"); err != nil {
				return err
			}
			if ga && item["role"] == "assistant" && p["type"] == "output_text" {
				p["type"] = "text"
			}
			if p["type"] != "input_text" && p["type"] != "text" {
				return errors.New("only text conversation items are supported")
			}
			if _, ok := p["text"].(string); !ok {
				return errors.New("text must be a string")
			}
		}
	default:
		return errors.New("unsupported conversation item type")
	}
	return nil
}

func (a *Adapter) gaSession(s map[string]any) error {
	if err := only(s, "type", "instructions", "output_modalities", "audio", "tools"); err != nil {
		return err
	}
	if s["type"] != "realtime" {
		return errors.New("session.type must be realtime")
	}
	delete(s, "type")
	if modalities, exists := s["output_modalities"]; exists {
		values, ok := modalities.([]any)
		if !ok || len(values) != 1 {
			return errors.New("select exactly one GA output modality")
		}
		switch values[0] {
		case "audio":
			s["modalities"] = []string{"text", "audio"}
		case "text":
			s["modalities"] = []string{"text"}
		default:
			return errors.New("unsupported output modality")
		}
		delete(s, "output_modalities")
	}
	if audio, exists := s["audio"]; exists {
		m, err := object(audio)
		if err != nil {
			return err
		}
		if err := only(m, "input", "output"); err != nil {
			return err
		}
		for _, direction := range []string{"input", "output"} {
			raw, ok := m[direction]
			if !ok {
				continue
			}
			config, err := object(raw)
			if err != nil {
				return err
			}
			if direction == "input" {
				if err := onlyAt("audio.input.", config, "format", "turn_detection", "transcription"); err != nil {
					return err
				}
			} else {
				if err := onlyAt("audio."+direction+".", config, "format", "voice"); err != nil {
					return err
				}
			}
			if format, exists := config["format"]; exists {
				f, err := object(format)
				if err != nil {
					return err
				}
				if err := only(f, "type", "rate"); err != nil {
					return err
				}
				if f["type"] != "audio/pcm" {
					return errors.New("Qwen GA adapter only supports PCM16")
				}
				if rate, exists := f["rate"]; exists && rate != float64(24000) {
					return errors.New("GA PCM requires 24000 Hz")
				}
				rate := 24000
				if direction == "input" && a.resampler != nil {
					rate = 16000
				}
				config["format"] = map[string]any{"type": "pcm", "sample_rate": rate}
			}
			if voice, exists := config["voice"]; exists {
				v, ok := voice.(string)
				if !ok || !a.cap.validVoice(v) {
					return errors.New("unsupported Qwen voice")
				}
				s["voice"] = v
				delete(config, "voice")
			}
			if turn, exists := config["turn_detection"]; exists {
				if err := validateTurn(turn, a.cap.VADTypes); err != nil {
					return err
				}
				s["turn_detection"] = turn
				delete(config, "turn_detection")
			}
			if transcription, exists := config["transcription"]; exists {
				if transcription != nil {
					tr, err := object(transcription)
					if err != nil {
						return err
					}
					if err := only(tr, "model"); err != nil {
						return err
					}
					if tr["model"] != "qwen3-asr-flash-realtime" {
						return errors.New("Qwen transcription requires qwen3-asr-flash-realtime")
					}
				}
				s["input_audio_transcription"] = transcription
				delete(config, "transcription")
			}
		}
	}
	if tools, exists := s["tools"]; exists {
		converted, err := convertTools(tools, true)
		if err != nil {
			return err
		}
		s["tools"] = converted
	}
	if v, exists := s["instructions"]; exists {
		if _, ok := v.(string); !ok {
			return errors.New("instructions must be text")
		}
	}
	return nil
}

func validateTurn(value any, allowedTypes []string) error {
	if value == nil {
		return nil
	}
	turn, err := object(value)
	if err != nil {
		return err
	}
	if err := only(turn, "type", "threshold", "prefix_padding_ms", "silence_duration_ms", "create_response", "interrupt_response"); err != nil {
		return err
	}
	if !slices.Contains(allowedTypes, fmt.Sprint(turn["type"])) {
		return errors.New("unsupported VAD type for this Realtime model")
	}
	for _, name := range []string{"create_response", "interrupt_response"} {
		if v, ok := turn[name]; ok {
			if _, ok := v.(bool); !ok {
				return errors.New("invalid VAD boolean")
			}
		}
	}
	if v, exists := turn["threshold"]; exists {
		n, ok := v.(float64)
		if !ok || n < -1 || n > 1 {
			return errors.New("invalid VAD threshold")
		}
	}
	for name, bounds := range map[string][2]int{"prefix_padding_ms": {0, 5000}, "silence_duration_ms": {200, 6000}} {
		if v, exists := turn[name]; exists {
			n, ok := number(v)
			if !ok || n < bounds[0] || n > bounds[1] {
				return errors.New("invalid VAD duration")
			}
		}
	}
	return nil
}

func convertTools(value any, ga bool) ([]any, error) {
	tools, ok := value.([]any)
	if !ok || len(tools) > 64 {
		return nil, errors.New("invalid tools")
	}
	result := make([]any, 0, len(tools))
	for _, tool := range tools {
		t, err := object(tool)
		if err != nil {
			return nil, err
		}
		if t["type"] != "function" {
			return nil, errors.New("only function tools are supported; remote MCP is disabled")
		}
		var f map[string]any
		if ga {
			if err := only(t, "type", "name", "description", "parameters"); err != nil {
				return nil, err
			}
			f = map[string]any{}
			for _, key := range []string{"name", "description", "parameters"} {
				if v, exists := t[key]; exists {
					f[key] = v
				}
			}
		} else {
			if err := only(t, "type", "function"); err != nil {
				return nil, err
			}
			f, err = object(t["function"])
			if err != nil {
				return nil, err
			}
			if err := only(f, "name", "description", "parameters"); err != nil {
				return nil, err
			}
		}
		name, ok := f["name"].(string)
		if !ok || name == "" || len(name) > 64 {
			return nil, errors.New("invalid function name")
		}
		if p, exists := f["parameters"]; exists {
			schema, err := object(p)
			if err != nil || schema["type"] != "object" {
				return nil, errors.New("function parameters require an object schema")
			}
		}
		result = append(result, map[string]any{"type": "function", "function": f})
	}
	return result, nil
}

var (
	ErrConflictingTurnDetection   = errors.New("session.audio.input.turn_detection conflicts with session.turn_detection")
	ErrInvalidNestedTurnDetection = errors.New("session.audio.input.turn_detection is invalid for the selected Qwen model")
)

func (a *Adapter) nativeSession(s map[string]any) error {
	if err := only(s, "modalities", "instructions", "voice", "audio", "turn_detection", "input_audio_transcription", "tools", "enable_search", "input_audio_format", "output_audio_format"); err != nil {
		return err
	}
	if value, exists := s["modalities"]; exists {
		m, ok := value.([]any)
		if !ok || len(m) == 0 || len(m) > 2 {
			return errors.New("unsupported native modalities")
		}
		text, audio := false, false
		for _, v := range m {
			switch v {
			case "text":
				text = true
			case "audio":
				audio = true
			default:
				return errors.New("unsupported native modality")
			}
		}
		if !text || (!audio && len(m) != 1) || (audio && len(m) != 2) {
			return errors.New("Qwen requires text or text and audio")
		}
	}
	if value, exists := s["instructions"]; exists {
		if _, ok := value.(string); !ok {
			return errors.New("instructions must be text")
		}
	}
	if value, exists := s["input_audio_transcription"]; exists && value != nil {
		m, err := object(value)
		if err != nil {
			return err
		}
		if err := only(m, "model"); err != nil {
			return err
		}
		if m["model"] != "qwen3-asr-flash-realtime" {
			return errors.New("unsupported native transcription model")
		}
	}
	if enabled, exists := s["enable_search"]; exists && enabled != false {
		return errors.New("search is disabled and cannot be combined with tools")
	}
	if tools, exists := s["tools"]; exists {
		if _, err := convertTools(tools, false); err != nil {
			return err
		}
	}
	if voice, exists := s["voice"]; exists {
		v, ok := voice.(string)
		if !ok || !a.cap.validVoice(v) {
			return errors.New("unsupported Qwen voice")
		}
	}
	if turn, exists := s["turn_detection"]; exists {
		if err := validateTurn(turn, a.cap.VADTypes); err != nil {
			return err
		}
	}
	for _, key := range []string{"input_audio_format", "output_audio_format"} {
		if v, ok := s[key]; ok && v != "pcm" {
			return errors.New("native safe subset only supports PCM16")
		}
	}
	if audio, exists := s["audio"]; exists {
		m, err := object(audio)
		if err != nil {
			return err
		}
		if err := only(m, "input", "output"); err != nil {
			return err
		}
		for _, direction := range []string{"input", "output"} {
			raw, exists := m[direction]
			if !exists {
				continue
			}
			config, err := object(raw)
			if err != nil {
				return err
			}
			allowed := []string{"format", "voice"}
			if direction == "input" {
				allowed = append(allowed, "turn_detection")
				if turn, exists := config["turn_detection"]; exists {
					if top, exists := s["turn_detection"]; exists && !reflect.DeepEqual(top, turn) {
						return ErrConflictingTurnDetection
					}
					if err := validateTurn(turn, a.cap.VADTypes); err != nil {
						return ErrInvalidNestedTurnDetection
					}
				}
			}
			if err := onlyAt("audio."+direction+".", config, allowed...); err != nil {
				return err
			}
			if format, exists := config["format"]; exists {
				f, err := object(format)
				if err != nil {
					return err
				}
				if err := only(f, "type", "sample_rate"); err != nil {
					return err
				}
				if f["type"] != "pcm" {
					return errors.New("only mono PCM16 is enabled")
				}
				n, ok := number(f["sample_rate"])
				supported := a.cap.supportsInputRate(int64(n))
				if direction == "output" {
					supported = a.cap.supportsOutputRate(int64(n))
				}
				if !ok || !supported {
					return errors.New("unsupported native sample rate")
				}
				if direction == "input" && !a.cap.FreeAudioTranscript && n != 16000 {
					return errors.New("Qwen3.8 native input requires 16000 Hz")
				}
				if a.mediaStarted && ((direction == "input" && n != a.inputRate) || (direction == "output" && n != a.outputRate)) {
					return errors.New("audio format cannot change after media input")
				}
				if direction == "input" {
					a.inputRate = n
				} else {
					a.outputRate = n
				}
			}
		}
	}
	return nil
}

var serverTypes = map[string]string{"session.created": "session.created", "session.updated": "session.updated", "input_audio_buffer.speech_started": "input_audio_buffer.speech_started", "input_audio_buffer.speech_stopped": "input_audio_buffer.speech_stopped", "input_audio_buffer.committed": "input_audio_buffer.committed", "input_audio_buffer.cleared": "input_audio_buffer.cleared", "conversation.item.created": "conversation.item.added", "conversation.item.input_audio_transcription.completed": "conversation.item.input_audio_transcription.completed", "conversation.item.input_audio_transcription.delta": "conversation.item.input_audio_transcription.delta", "conversation.item.input_audio_transcription.failed": "conversation.item.input_audio_transcription.failed", "response.created": "response.created", "response.output_item.added": "response.output_item.added", "response.output_item.done": "response.output_item.done", "response.content_part.added": "response.content_part.added", "response.content_part.done": "response.content_part.done", "response.audio.delta": "response.output_audio.delta", "response.audio.done": "response.output_audio.done", "response.audio_transcript.delta": "response.output_audio_transcript.delta", "response.audio_transcript.done": "response.output_audio_transcript.done", "response.text.delta": "response.output_text.delta", "response.text.done": "response.output_text.done", "response.function_call_arguments.delta": "response.function_call_arguments.delta", "response.function_call_arguments.done": "response.function_call_arguments.done", "response.done": "response.done", "error": "error", "rate_limits.updated": "rate_limits.updated"}

func (a *Adapter) Server(data []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	event, err := decode(data)
	if err != nil {
		return nil, err
	}
	e := event.Fields
	if event.Type == "error" {
		if id := qwenErrorEventID(e["error"]); id == a.pendingUpdateID && id != "" {
			a.inputRate = a.confirmedInput
			a.outputRate = a.confirmedOutput
			a.pendingUpdateID = ""
		}
		redactQwenError(e, "invalid_request_error", "The Qwen Realtime service rejected an event.")
		envelope := map[string]any{"type": "error", "error": e["error"]}
		if id, ok := e["event_id"].(string); ok && id != "" && len(id) <= 256 {
			envelope["event_id"] = id
		}
		return json.Marshal(envelope)
	}
	if event.Type == "conversation.item.input_audio_transcription.failed" {
		redactQwenError(e, "server_error", "The Qwen Realtime transcription failed.")
	}
	if a.native {
		if event.Type == "conversation.item.input_audio_transcription.failed" {
			return json.Marshal(e)
		}
		if event.Type == "session.updated" {
			s, err := object(e["session"])
			if err != nil {
				return nil, err
			}
			if err := a.verifyAudioAck(s); err != nil {
				return nil, err
			}
		}
		if event.Type == "response.done" {
			response, _ := e["response"].(map[string]any)
			if redactQwenResponseError(response) {
				return json.Marshal(e)
			}
		}
		return data, nil
	}
	mapped, ok := serverTypes[event.Type]
	if !ok {
		return nil, errors.New("unsupported Qwen server event")
	}
	e["type"] = mapped
	if event.Type == "session.created" || event.Type == "session.updated" {
		session, err := object(e["session"])
		if err != nil {
			return nil, err
		}
		if event.Type == "session.updated" {
			if err := a.verifyAudioAck(session); err != nil {
				return nil, err
			}
		}
		e["session"] = gaSessionFromQwen(session)
	}
	if item, ok := e["item"].(map[string]any); ok {
		convertItem(item)
	}
	if part, ok := e["part"].(map[string]any); ok {
		convertContent(part)
	}
	if response, ok := e["response"].(map[string]any); ok {
		redactQwenResponseError(response)
		if response["status"] == "canceled" {
			response["status"] = "cancelled"
		}
		if modalities, ok := response["modalities"].([]any); ok {
			response["output_modalities"] = gaModalities(modalities)
			delete(response, "modalities")
		}
		delete(response, "voice")
		delete(response, "output_audio_format")
		if items, ok := response["output"].([]any); ok {
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					convertItem(m)
				}
			}
		}
		status, _ := response["status"].(string)
		if event.Type == "response.done" && (status != "cancelled" || response["usage"] != nil) {
			if _, usage, err := ParseUsage(data); err == nil {
				response["usage"] = gaUsage(usage)
			} else {
				return nil, err
			}
		}
	}
	if event.Type == "response.audio.delta" {
		audio, ok := e["delta"].(string)
		if !ok {
			return nil, errors.New("audio delta missing")
		}
		pcm, err := base64.StdEncoding.Strict().DecodeString(audio)
		if err != nil || len(pcm)%2 != 0 {
			return nil, errors.New("invalid upstream PCM audio")
		}
	}
	return json.Marshal(e)
}

func qwenErrorEventID(value any) string {
	upstream, _ := value.(map[string]any)
	id, _ := upstream["event_id"].(string)
	return id
}

// SafeParameterDiagnostic recognizes provider diagnostics that contain only a
// parameter path. Arbitrary provider messages may echo input or credentials.
func SafeParameterDiagnostic(code, message string) bool {
	return (code == "unknown_parameter" || code == "invalid_value" || code == "invalid_parameter") &&
		realtimeParameterDiagnostic.MatchString(message)
}

var realtimeParameterDiagnostic = regexp.MustCompile(`^Unknown parameter: '[a-zA-Z_][a-zA-Z0-9_]{0,63}(\.[a-zA-Z_][a-zA-Z0-9_]{0,63}){0,7}'\.?$`)

func redactQwenError(event map[string]any, kind, message string) {
	safe := map[string]any{"type": kind, "code": "upstream_error", "message": message}
	upstream, _ := event["error"].(map[string]any)
	code, _ := upstream["code"].(string)
	detail, _ := upstream["message"].(string)
	if kind == "invalid_request_error" && SafeParameterDiagnostic(code, detail) {
		safe["code"], safe["message"] = code, detail
	}
	if id := qwenErrorEventID(event["error"]); id != "" && len(id) <= 256 {
		safe["event_id"] = id
	}
	event["error"] = safe
}

// redactQwenResponseError reports whether the response needs to be serialized again.
func redactQwenResponseError(response map[string]any) bool {
	details, _ := response["status_details"].(map[string]any)
	if _, exists := details["error"]; !exists {
		return false
	}
	details["error"] = map[string]any{"type": "server_error", "code": "upstream_error", "message": "The Qwen Realtime response failed."}
	return true
}

func (a *Adapter) verifyAudioAck(session map[string]any) error {
	a.pendingUpdateID = ""
	if !a.cap.FreeAudioTranscript {
		if _, nested := session["audio"]; !nested && session["input_audio_format"] == "pcm" && session["output_audio_format"] == "pcm" {
			a.confirmedInput = a.inputRate
			a.confirmedOutput = a.outputRate
			return nil
		}
	}

	audio, err := object(session["audio"])
	if err != nil {
		return errors.New("Qwen did not acknowledge negotiated PCM rates")
	}
	for _, direction := range []string{"input", "output"} {
		config, err := object(audio[direction])
		if a.native && !a.cap.FreeAudioTranscript && session[direction+"_audio_format"] == "pcm" {
			_, directionPresent := audio[direction]
			_, formatPresent := config["format"]
			if !directionPresent || (err == nil && !formatPresent) {
				continue
			}
		}
		if err != nil {
			return err
		}
		f, err := object(config["format"])
		if err != nil {
			return err
		}
		expected := 24000
		if direction == "input" {
			expected = a.inputRate
		}
		if a.native && direction == "output" {
			expected = a.outputRate
		}
		if f["type"] != "pcm" || f["sample_rate"] != float64(expected) {
			return errors.New("Qwen did not acknowledge the negotiated audio format")
		}
	}
	a.confirmedInput = a.inputRate
	a.confirmedOutput = a.outputRate
	return nil
}

func gaModalities(values []any) []string {
	for _, v := range values {
		if v == "audio" {
			return []string{"audio"}
		}
	}
	return []string{"text"}
}

func gaSessionFromQwen(s map[string]any) map[string]any {
	result := map[string]any{"type": "realtime", "audio": map[string]any{"input": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}, "turn_detection": s["turn_detection"], "transcription": s["input_audio_transcription"]}, "output": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}, "voice": s["voice"]}}}
	for _, key := range []string{"id", "object", "model", "instructions"} {
		if v, ok := s[key]; ok {
			result[key] = v
		}
	}
	if m, ok := s["modalities"].([]any); ok {
		result["output_modalities"] = gaModalities(m)
	}
	if tools, ok := s["tools"].([]any); ok {
		result["tools"] = []any{}
		for _, t := range tools {
			if m, ok := t.(map[string]any); ok {
				if f, ok := m["function"].(map[string]any); ok {
					f["type"] = "function"
					result["tools"] = append(result["tools"].([]any), f)
				}
			}
		}
	}
	return result
}

func convertContent(part map[string]any) {
	switch part["type"] {
	case "audio", "input_audio":
		part["type"] = "output_audio"
	case "text", "input_text":
		part["type"] = "output_text"
	}
}

func convertItem(item map[string]any) {
	if content, ok := item["content"].([]any); ok {
		for _, part := range content {
			if p, ok := part.(map[string]any); ok {
				if item["role"] == "assistant" {
					convertContent(p)
				} else if item["role"] == "user" {
					switch p["type"] {
					case "audio":
						p["type"] = "input_audio"
					case "text":
						p["type"] = "input_text"
					}
				}
			}
		}
	}
}

func gaUsage(u *llm.Usage) map[string]any {
	return map[string]any{"input_tokens": u.PromptTokens, "output_tokens": u.CompletionTokens, "total_tokens": u.TotalTokens, "input_token_details": map[string]any{"text_tokens": u.PromptTokens - u.PromptTokensDetails.AudioTokens, "audio_tokens": u.PromptTokensDetails.AudioTokens, "cached_tokens": 0}, "output_token_details": map[string]any{"text_tokens": u.CompletionTokens - u.CompletionTokensDetails.AudioTokens, "audio_tokens": u.CompletionTokensDetails.AudioTokens}}
}

func ParseUsage(data []byte) (string, *llm.Usage, error) {
	event, err := decode(data)
	if err != nil {
		return "", nil, err
	}
	if event.Type != "response.done" {
		return "", nil, nil
	}
	response, err := object(event.Fields["response"])
	if err != nil {
		return "", nil, err
	}
	usage, err := object(response["usage"])
	if err != nil {
		return "", nil, errors.New("Qwen usage is unknown")
	}
	usage["input_token_details"] = usage["input_tokens_details"]
	usage["output_token_details"] = usage["output_tokens_details"]
	delete(usage, "input_tokens_details")
	delete(usage, "output_tokens_details")
	wire, err := json.Marshal(event.Fields)
	if err != nil {
		return "", nil, err
	}
	return nativega.ParseUsage(wire)
}

// Qwen3.8's established PCM contract is 16k input / 24k output; use the documented legacy fields.
func flattenFixedPCM(session map[string]any) {
	audio, ok := session["audio"].(map[string]any)
	if !ok {
		return
	}
	for _, direction := range []string{"input", "output"} {
		config, ok := audio[direction].(map[string]any)
		if !ok {
			continue
		}
		if _, exists := config["format"]; exists {
			session[direction+"_audio_format"] = "pcm"
			delete(config, "format")
		}
		if voice, exists := config["voice"]; exists {
			session["voice"] = voice
			delete(config, "voice")
		}
		if len(config) == 0 {
			delete(audio, direction)
		}
	}
	if len(audio) == 0 {
		delete(session, "audio")
	}
}
