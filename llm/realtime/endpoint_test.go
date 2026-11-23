// 职责: 覆盖 Realtime 端点白名单校验与 URL 构造，阻断 host/端口/绕过注入。
// 关联文件:
//
//	llm/realtime/endpoint.go
//	llm/realtime/capability.go
package realtime

import (
	"strings"
	"testing"
)

func TestValidateEndpointValid(t *testing.T) {
	cases := []struct {
		base string
	}{
		{"wss://ws-abc123.cn-beijing.maas.aliyuncs.com"},
		{"wss://workspace-1.ap-southeast-1.maas.aliyuncs.com"},
		{"wss://a.cn-beijing.maas.aliyuncs.com/"},
	}
	for _, c := range cases {
		if err := ValidateEndpoint(c.base, qwenRealtimePath); err != nil {
			t.Errorf("ValidateEndpoint(%q) = %v, want nil", c.base, err)
		}
	}
}

func TestValidateEndpointRejectsBadScheme(t *testing.T) {
	for _, base := range []string{
		"ws://a.cn-beijing.maas.aliyuncs.com",
		"https://a.cn-beijing.maas.aliyuncs.com",
		"http://a.cn-beijing.maas.aliyuncs.com",
		"ftp://a.cn-beijing.maas.aliyuncs.com",
		"//a.cn-beijing.maas.aliyuncs.com",
		"",
	} {
		if err := ValidateEndpoint(base, qwenRealtimePath); err == nil {
			t.Errorf("ValidateEndpoint(%q) = nil, want error", base)
		}
	}
}

func TestValidateEndpointRejectsPortUserQueryFragment(t *testing.T) {
	bad := []string{
		"wss://a.cn-beijing.maas.aliyuncs.com:443",
		"wss://user:pass@a.cn-beijing.maas.aliyuncs.com",
		"wss://user@a.cn-beijing.maas.aliyuncs.com",
		"wss://a.cn-beijing.maas.aliyuncs.com?x=1",
		"wss://a.cn-beijing.maas.aliyuncs.com/?x=1",
		"wss://a.cn-beijing.maas.aliyuncs.com/#frag",
		"wss://a.cn-beijing.maas.aliyuncs.com/#",
	}
	for _, base := range bad {
		if err := ValidateEndpoint(base, qwenRealtimePath); err == nil {
			t.Errorf("ValidateEndpoint(%q) = nil, want error", base)
		}
	}
}

func TestValidateEndpointRejectsHostBypass(t *testing.T) {
	bad := []string{
		"wss://a.cn-beijing.maas.aliyuncs.com.evil.com",
		"wss://evil.com",
		"wss://a.cn-beijing.maas.aliyuncs.com..",
		"wss://a.cn-beijing.maas.aliyuncs.com.",
		"wss://a.cn-beijingx.maas.aliyuncs.com",
		"wss://a..cn-beijing.maas.aliyuncs.com",
		"wss://.cn-beijing.maas.aliyuncs.com",
		"wss://a.b.cn-beijing.maas.aliyuncs.com",
		"wss://a%2ecn-beijing.maas.aliyuncs.com",
		"wss://a\\cn-beijing.maas.aliyuncs.com",
		"wss://127.0.0.1",
		"wss://localhost",
		"wss://a.cn-beijing.maas.aliyuncs.com@evil.com",
	}
	for _, base := range bad {
		if err := ValidateEndpoint(base, qwenRealtimePath); err == nil {
			t.Errorf("ValidateEndpoint(%q) = nil, want error", base)
		}
	}
}

func TestValidateEndpointRejectsPath(t *testing.T) {
	base := "wss://a.cn-beijing.maas.aliyuncs.com"
	bad := []string{
		"",
		"/api-ws/v1/realtime/",
		"/api-ws/v1/realtime?x=1",
		"/api-ws/v1/realtime#frag",
		"/api-ws/v1/../v1/realtime",
		"/api-ws/v1/realtime/extra",
		"api-ws/v1/realtime",
	}
	for _, p := range bad {
		if err := ValidateEndpoint(base, p); err == nil {
			t.Errorf("ValidateEndpoint(path=%q) = nil, want error", p)
		}
	}
}

func TestEndpointURL(t *testing.T) {
	got, err := EndpointURL(
		"wss://ws-abc.cn-beijing.maas.aliyuncs.com",
		qwenRealtimePath,
		"qwen3.8-omni-flash-realtime",
	)
	if err != nil {
		t.Fatalf("EndpointURL error: %v", err)
	}
	want := "wss://ws-abc.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime?model=qwen3.8-omni-flash-realtime"
	if got != want {
		t.Fatalf("EndpointURL = %q, want %q", got, want)
	}
	if strings.Contains(got, "..") {
		t.Fatalf("unexpected dot in %q", got)
	}
}

func TestEndpointURLRejectsUnknownModel(t *testing.T) {
	for _, m := range []string{"", "gpt-realtime", "qwen3.8-omni-flash-realtime ", "qwen3.8-omni-flash-realtime&x=1"} {
		if _, err := EndpointURL("wss://a.cn-beijing.maas.aliyuncs.com", qwenRealtimePath, m); err == nil {
			t.Errorf("EndpointURL(model=%q) = nil, want error", m)
		}
	}
}

func TestEndpointURLRejectsBadBase(t *testing.T) {
	if _, err := EndpointURL("wss://a.cn-beijing.maas.aliyuncs.com:443", qwenRealtimePath, "qwen3.5-omni-plus-realtime"); err == nil {
		t.Error("expected port rejection")
	}
	if _, err := EndpointURL("ws://a.cn-beijing.maas.aliyuncs.com", qwenRealtimePath, "qwen3.5-omni-plus-realtime"); err == nil {
		t.Error("expected ws scheme rejection")
	}
}
