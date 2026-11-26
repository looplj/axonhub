// 职责: 通过已配置的 AxonHub Gateway 验证实时文本、PCM 音频、多轮与工具调用。
// 关联文件:
//
//	internal/server/api/realtime.go
//	docs/en/guides/realtime.md
//	docs/zh/guides/realtime.md
//
// 注意事项:
//
//	凭证仅从 AXONHUB_API_KEY 环境读取；只输出事件类型和计量，不输出媒体、对话或密钥。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coder/websocket"

	protocol "github.com/looplj/axonhub/llm/realtime"
)

var (
	model     = flag.String("model", "gpt-realtime-2.1-mini", "Explicitly configured Gateway model or alias")
	endpoint  = flag.String("url", "ws://127.0.0.1:8090/v1/realtime", "Gateway WebSocket URL without query credentials")
	audioFile = flag.String("audio-pcm", "", "Headerless, mono signed PCM16 LE at 24000 Hz; required for audio acceptance")
	native    = flag.Bool("qwen-native", false, "Use the explicitly enabled /qwen/v1/realtime route")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Realtime smoke FAILED:", err)
		os.Exit(1)
	}
}

//nolint:maintidx // Keep this standalone smoke flow in its observed protocol order.
func run() error {
	key := os.Getenv("AXONHUB_API_KEY")
	if key == "" {
		return errors.New("AXONHUB_API_KEY is required; no live checks executed")
	}
	audio, err := os.ReadFile(*audioFile)
	if err != nil || len(audio) == 0 || len(audio)%2 != 0 {
		return errors.New("-audio-pcm must supply a nonempty mono PCM16 24 kHz recording; no live checks executed")
	}
	if *native && protocol.NativeInputRate(*model) == 16000 {
		audio, err = protocol.NewPCM24To16().Append(audio)
		if err != nil {
			return err
		}
	}
	u, err := url.Parse(*endpoint)
	if err != nil || u.Host == "" || u.User != nil {
		return errors.New("invalid gateway URL")
	}
	if *native {
		u.Path = "/qwen/v1/realtime"
	}
	u.RawQuery = url.Values{"model": []string{*model}}.Encode()
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelDial()
	conn, resp, err := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + key}},
	})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			return fmt.Errorf("gateway handshake status %d", resp.StatusCode)
		}
		return errors.New("gateway handshake failed")
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(256 << 10)
	send := func(v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		writeCtx, cancelWrite := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelWrite()
		return conn.Write(writeCtx, websocket.MessageText, data)
	}
	wait := func(want string) (map[string]any, error) {
		readCtx, cancelRead := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancelRead()
		for range 10000 {
			_, data, err := conn.Read(readCtx)
			if err != nil {
				return nil, errors.New("socket closed or timed out before " + want)
			}
			var event map[string]any
			if json.Unmarshal(data, &event) != nil {
				return nil, errors.New("invalid event JSON")
			}
			kind, _ := event["type"].(string)
			if kind == "error" {
				return nil, errors.New("upstream/gateway rejected an event; inspect sanitized Gateway audit metadata")
			}
			if kind == "response.done" {
				body, _ := event["response"].(map[string]any)
				if err := checkCompletedResponse(body); err != nil {
					return nil, err
				}
				usage, _ := json.Marshal(body["usage"])
				fmt.Println("confirmed usage", string(usage))
			}
			if kind == want {
				return event, nil
			}
		}
		return nil, errors.New("event count limit exceeded")
	}
	if _, err := wait("session.created"); err != nil {
		return err
	}
	update := func(audioOutput bool, tools bool) error {
		modalities := []string{"text"}
		if audioOutput {
			modalities = []string{"audio"}
		}
		session := map[string]any{"type": "realtime", "output_modalities": modalities, "audio": map[string]any{"input": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}, "turn_detection": nil}, "output": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}}}}
		if tools {
			session["tools"] = []any{map[string]any{"type": "function", "name": "gateway_smoke", "description": "Call this once to verify the gateway, then use the returned result.", "parameters": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}}
		}
		if *native {
			delete(session, "type")
			delete(session, "output_modalities")
			delete(session, "audio")
			session["modalities"] = []string{"text"}
			if audioOutput {
				session["modalities"] = []string{"text", "audio"}
			}
			session["turn_detection"] = nil
			session["audio"] = map[string]any{"input": map[string]any{"format": map[string]any{"type": "pcm", "sample_rate": protocol.NativeInputRate(*model)}}, "output": map[string]any{"format": map[string]any{"type": "pcm", "sample_rate": 24000}}}
			if tools {
				session["tools"] = []any{map[string]any{"type": "function", "function": map[string]any{"name": "gateway_smoke", "description": "Call this once to verify the gateway, then use the returned result.", "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}}}
			}
		}
		if err := send(map[string]any{"type": "session.update", "session": session}); err != nil {
			return err
		}
		_, err := wait("session.updated")
		return err
	}
	text := func(content string) error {
		return send(map[string]any{"type": "conversation.item.create", "item": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": content}}}})
	}
	if err := update(false, false); err != nil {
		return err
	}
	if err := text("Reply with a short greeting."); err != nil {
		return err
	}
	if err := send(map[string]any{"type": "response.create"}); err != nil {
		return err
	}
	if _, err := wait("response.done"); err != nil {
		return err
	}
	fmt.Println("PASS text turn")
	if err := update(true, false); err != nil {
		return err
	}
	for offset := 0; offset < len(audio); offset += 24000 {
		end := min(offset+24000, len(audio))
		if err := send(map[string]any{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(audio[offset:end])}); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := send(map[string]any{"type": "input_audio_buffer.commit"}); err != nil {
		return err
	}
	if _, err := wait("input_audio_buffer.committed"); err != nil {
		return err
	}
	if err := send(map[string]any{"type": "response.create"}); err != nil {
		return err
	}
	kind := "response.output_audio.delta"
	if *native {
		kind = "response.audio.delta"
	}
	if _, err := wait(kind); err != nil {
		return err
	}
	if _, err := wait("response.done"); err != nil {
		return err
	}
	fmt.Println("PASS audio input/output and continuous second turn; manually review intelligibility/duration separately")
	if err := update(false, true); err != nil {
		return err
	}
	if err := text("Call gateway_smoke exactly once now; do not answer until you receive its result."); err != nil {
		return err
	}
	if err := send(map[string]any{"type": "response.create"}); err != nil {
		return err
	}
	args, err := wait("response.function_call_arguments.done")
	if err != nil {
		return err
	}
	callID, _ := args["call_id"].(string)
	if callID == "" {
		return errors.New("function call_id missing")
	}
	if _, err := wait("response.done"); err != nil {
		return err
	}
	if err := send(map[string]any{"type": "conversation.item.create", "item": map[string]any{"type": "function_call_output", "call_id": callID, "output": "{\"ok\":true}"}}); err != nil {
		return err
	}
	if err := send(map[string]any{"type": "response.create"}); err != nil {
		return err
	}
	if _, err := wait("response.done"); err != nil {
		return err
	}
	fmt.Println("PASS function call, result and continuation")
	return conn.Close(websocket.StatusNormalClosure, "smoke complete")
}

func checkCompletedResponse(body map[string]any) error {
	if body["status"] != "completed" {
		return errors.New("response.done did not report completed status")
	}
	if body["usage"] == nil {
		return errors.New("response.done has unknown usage")
	}
	return nil
}
