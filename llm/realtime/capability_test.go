// 职责: 覆盖 Realtime 数据驱动能力表、安全子集与模型校验。
// 关联文件:
//
//	llm/realtime/capability.go
//	llm/realtime/adapter.go
package realtime

import "testing"

func TestAssociateModelsClosedSet(t *testing.T) {
	want := []string{
		"qwen3.8-omni-flash-realtime",
		"qwen3.5-omni-plus-realtime",
		"qwen3.5-omni-flash-realtime",
	}
	for _, m := range want {
		if !IsAssociatedModel(m) {
			t.Errorf("expected %q to be associated", m)
		}
	}
	for _, m := range []string{"gpt-realtime-2", "other", "", "qwen3-omni-flash-realtime-2025-09-15"} {
		if IsAssociatedModel(m) {
			t.Errorf("expected %q to be rejected", m)
		}
	}
	if len(modelNames()) != len(want) {
		t.Errorf("capability set size = %d, want %d", len(modelNames()), len(want))
	}
}

func TestComposeCapabilitySafeSubset(t *testing.T) {
	// The safe subset is mono PCM16 24kHz input / pcm output. All three
	// associated models advertise a rate that matches the safe subset.
	for m := range associateModels {
		cap, safe := ComposeCapability(m)
		if safe != (m != "qwen3.8-omni-flash-realtime") {
			t.Errorf("model %q has an incorrect identity-format negotiation result", m)
		}
		if !cap.supportsNestedAudio() {
			t.Errorf("model %q must support nested session.audio config", m)
		}
		if !cap.supportsInputFormat("pcm") || !cap.supportsInputFormat("wav") {
			t.Errorf("model %q input formats too narrow", m)
		}
	}
	if _, safe := ComposeCapability("unsupported-model"); safe {
		t.Error("unsupported model must not be safe")
	}
	n := NewSafeSubset()
	if n.InputFormat != "pcm" || n.InputSampleRate != 24000 {
		t.Errorf("safe input = %s/%d, want pcm/24000", n.InputFormat, n.InputSampleRate)
	}
	if n.OutputFormat != "pcm" || n.OutputSampleRate != 24000 {
		t.Errorf("safe output = %s/%d, want pcm/24000", n.OutputFormat, n.OutputSampleRate)
	}
}

func TestCapabilityRateStrictness(t *testing.T) {
	cap := capabilityIndex["qwen3.5-omni-flash-realtime"]
	for _, r := range []int64{8000, 16000, 24000, 48000} {
		if !cap.supportsInputRate(r) {
			t.Errorf("expected input rate %d supported", r)
		}
		if !cap.supportsOutputRate(r) {
			t.Errorf("expected output rate %d supported", r)
		}
	}
	if cap.supportsInputRate(12345) {
		t.Error("unexpected rate must be rejected")
	}
}
