// 职责: 按 Qwen 模型描述已验证的实时音频契约与计费差异。
// 关联文件:
//
//	llm/realtime/adapter.go
//	llm/realtime/endpoint.go
//
// 注意事项:
//
//	单声道 PCM16 可协商 24 kHz，GA 不修改音频字节或伪造采样率。
package realtime

import (
	"fmt"
	"slices"
	"sort"
)

type capability struct {
	inputAudioFormats                   map[string]struct{}
	inputSampleRates, outputSampleRates map[int64]struct{}
	nestedAudioSupport                  bool
	FreeAudioTranscript                 bool
	Voices                              []string
	Regions                             []string
	VADTypes                            []string
	Video, MCP, Search, Multichannel    bool
	FunctionCalling                     bool
}

var (
	associateModels = map[string]struct{}{"qwen3.8-omni-flash-realtime": {}, "qwen3.5-omni-plus-realtime": {}, "qwen3.5-omni-flash-realtime": {}}
	capabilityIndex = func() map[string]capability {
		result := map[string]capability{}
		for model := range associateModels {
			result[model] = capability{inputAudioFormats: map[string]struct{}{"pcm": {}, "wav": {}}, inputSampleRates: map[int64]struct{}{8000: {}, 16000: {}, 24000: {}, 48000: {}}, outputSampleRates: map[int64]struct{}{8000: {}, 16000: {}, 24000: {}, 48000: {}}, nestedAudioSupport: true, FreeAudioTranscript: model != "qwen3.8-omni-flash-realtime", Voices: []string{"Tina", "Cherry", "Ethan", "Chelsie", "Serena"}, Regions: []string{"cn-beijing", "ap-southeast-1"}, VADTypes: []string{"server_vad", "semantic_vad"}, FunctionCalling: true}
		}
		fixed := result["qwen3.8-omni-flash-realtime"]
		fixed.inputSampleRates = map[int64]struct{}{16000: {}}
		fixed.outputSampleRates = map[int64]struct{}{24000: {}}
		result["qwen3.8-omni-flash-realtime"] = fixed
		return result
	}()
)

func (c capability) supportsInputFormat(format string) bool {
	_, ok := c.inputAudioFormats[format]
	return ok
}

func (c capability) supportsInputRate(rate int64) bool { _, ok := c.inputSampleRates[rate]; return ok }

func (c capability) supportsOutputRate(rate int64) bool {
	_, ok := c.outputSampleRates[rate]
	return ok
}
func (c capability) supportsNestedAudio() bool { return c.nestedAudioSupport }
func IsAssociatedModel(model string) bool      { _, ok := capabilityIndex[model]; return ok }
func FreeAudioTranscript(model string) bool    { return capabilityIndex[model].FreeAudioTranscript }
func modelNames() []string {
	result := make([]string, 0, len(associateModels))
	for model := range associateModels {
		result = append(result, model)
	}
	sort.Strings(result)
	return result
}

func errUnsupportedModel(model string) error {
	return fmt.Errorf("unsupported Qwen realtime model %q", model)
}

type SafeSubset struct {
	InputFormat, OutputFormat         string
	InputSampleRate, OutputSampleRate int64
}

func NewSafeSubset() SafeSubset {
	return SafeSubset{InputFormat: "pcm", OutputFormat: "pcm", InputSampleRate: 24000, OutputSampleRate: 24000}
}

func ComposeCapability(model string) (capability, bool) {
	c, ok := capabilityIndex[model]
	return c, ok && c.supportsInputRate(24000) && c.supportsOutputRate(24000)
}

func (c capability) validVoice(voice string) bool {
	return slices.Contains(c.Voices, voice) || (voice == "longanlingxin" && !c.FreeAudioTranscript)
}

func NativeInputRate(model string) int {
	if model == "qwen3.8-omni-flash-realtime" {
		return 16000
	}
	return 24000
}
