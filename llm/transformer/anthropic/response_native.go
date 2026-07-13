package anthropic

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

// applyAnthropicResponseNativeFields restores Anthropic-only response fields that
// are not represented on the common llm.Response model.
func applyAnthropicResponseNativeFields(resp *Message, chatResp *llm.Response) {
	if resp == nil || chatResp == nil {
		return
	}

	if chatResp.ProviderExtensions == nil || chatResp.ProviderExtensions.Anthropic == nil ||
		chatResp.ProviderExtensions.Anthropic.Response == nil {
		return
	}

	native := chatResp.ProviderExtensions.Anthropic.Response
	if native.StopSequence != nil {
		seq := *native.StopSequence
		resp.StopSequence = &seq
	}
	resp.StopDetails = append(json.RawMessage(nil), native.StopDetails...)

	// Original usage object, including server-tool / future detail children.
	if len(native.RawUsage) > 0 {
		if resp.Usage == nil {
			resp.Usage = &Usage{}
		}
		resp.Usage.Raw = append(json.RawMessage(nil), native.RawUsage...)
	}
}
