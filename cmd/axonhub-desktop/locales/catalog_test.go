package locales

import "testing"

func TestCatalogSpeaksChinese(t *testing.T) {
	if got := Settings(); got != "Settings" {
		t.Fatalf("source language Settings() = %q", got)
	}
	zh := In("zh-Hans")
	if got := zh.Settings(); got != "设置" {
		t.Fatalf("zh-Hans Settings() = %q", got)
	}
	if got := zh.RefreshesEvery("5"); got != "每 5 秒" {
		t.Fatalf("zh-Hans RefreshesEvery() = %q", got)
	}
	if got := zh.Port(); got != "端口" {
		t.Fatalf("zh-Hans Port() = %q", got)
	}
	if got := zh.PortInvalid(); got != "端口必须在 1 到 65535 之间" {
		t.Fatalf("zh-Hans PortInvalid() = %q", got)
	}
	if got := zh.UpdateTo("1.2.3"); got != "更新到 1.2.3…" {
		t.Fatalf("zh-Hans UpdateTo() = %q", got)
	}
}
