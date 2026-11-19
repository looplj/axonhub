// 职责: 验证实时协议与既有 HTTP 请求类型隔离。
// 关联文件:
//
//	llm/constants.go
//	llm/api_capability.go
package llm

import "testing"

func TestRealtimeCapabilityIsolation(t *testing.T) {
	if _, ok := CapableAPIFormats(RequestTypeRealtime)[APIFormatQwenRealtimeWebSocket.String()]; !ok {
		t.Fatal("Realtime must select Qwen realtime")
	}
	for _, kind := range []RequestType{RequestTypeChat, RequestTypeSpeech, RequestTypeTranscription} {
		if _, ok := CapableAPIFormats(kind)[APIFormatQwenRealtimeWebSocket.String()]; ok {
			t.Fatalf("%s can route to realtime", kind)
		}
	}
	for _, format := range []APIFormat{APIFormatOpenAIChatCompletion, APIFormatOpenAIResponse, APIFormatOpenAIResponseWebSocket} {
		if _, ok := CapableAPIFormats(RequestTypeRealtime)[format.String()]; ok {
			t.Fatalf("realtime can route to %s", format)
		}
	}
}
