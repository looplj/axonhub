// 职责: 验证 Channels 页的数据层：渠道映射、状态标签和测试结果。
// 关联文件:
//   cmd/axonhub-desktop/panel_channels.go
// 遵循的 Rules:
//   .agents/rules/go-general.md

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadPanelChannelsMapsRows(t *testing.T) {
	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("X-Project-ID")
		_, _ = w.Write([]byte(`{"data":{"channels":{"edges":[
			{"node":{"id":"c1","name":"Anthropic","type":"anthropic","status":"enabled","supportedModels":["claude-a","claude-b"],"providerQuotaStatus":{"status":"warning"}}},
			{"node":{"id":"c2","name":"Backup","type":"openai","status":"disabled","errorMessage":" 401 unauthorized ","autoDisabledAt":"2026-01-02T03:04:05Z","supportedModels":[]}}
		]}}}`))
	}))
	defer server.Close()

	channels, err := loadPanelChannels(t.Context(), server.Client(), server.URL, "desktop-token")
	if err != nil {
		t.Fatal(err)
	}
	if header != "" {
		t.Fatalf("X-Project-ID = %q, want none", header)
	}
	if len(channels) != 2 {
		t.Fatalf("channels = %#v", channels)
	}
	first, second := channels[0], channels[1]
	if !first.Enabled || first.Quota != quotaWarning || len(first.Models) != 2 || first.AutoDisabled {
		t.Fatalf("first = %#v", first)
	}
	if second.Enabled || !second.AutoDisabled || second.Error != "401 unauthorized" || second.Quota != "" {
		t.Fatalf("second = %#v", second)
	}
}

func TestLoadPanelChannelsReadsTheNextPage(t *testing.T) {
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted, err := readPostedGraphQL(r)
		if err != nil {
			t.Error(err)
		}
		after, _ := posted.Variables["after"].(string)
		cursors = append(cursors, after)
		if after == "" {
			_, _ = w.Write([]byte(`{"data":{"channels":{"edges":[{"node":{"id":"c1","name":"A","type":"openai","status":"enabled","supportedModels":[]}}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"channels":{"edges":[{"node":{"id":"c2","name":"B","type":"openai","status":"disabled","supportedModels":[]}}],"pageInfo":{"hasNextPage":false}}}}`))
	}))
	defer server.Close()

	channels, err := loadPanelChannels(t.Context(), server.Client(), server.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 || channels[0].ID != "c1" || channels[1].ID != "c2" || channels[1].Enabled {
		t.Fatalf("channels = %#v", channels)
	}
	if len(cursors) != 2 || cursors[1] != "c1" {
		t.Fatalf("cursors = %#v", cursors)
	}
}

func TestMergeBusyChannelsKeepsTheSwitch(t *testing.T) {
	current := []panelChannel{{ID: "c1", Enabled: true, AutoDisabled: false}}
	incoming := []panelChannel{{ID: "c1", Enabled: false, AutoDisabled: true, Name: "Main"}}
	got := mergeBusyChannels(incoming, current, map[string]bool{"c1": true})
	if !got[0].Enabled || got[0].AutoDisabled || got[0].Name != "Main" {
		t.Fatalf("merged = %#v", got)
	}
}

func TestChannelStatus(t *testing.T) {
	tests := []struct {
		channel panelChannel
		kind    string
		tone    statusTone
	}{
		{panelChannel{Enabled: true}, "", toneMuted},
		{panelChannel{Enabled: true, Quota: quotaWarning}, "quota_warning", toneActive},
		{panelChannel{Enabled: true, Quota: quotaExhausted}, "quota_exhausted", toneDanger},
		{panelChannel{Quota: quotaExhausted}, "disabled", toneMuted},
		{panelChannel{AutoDisabled: true}, "auto_disabled", toneDanger},
	}
	for _, tt := range tests {
		if kind, tone := channelStatus(tt.channel); kind != tt.kind || tone != tt.tone {
			t.Errorf("channelStatus(%#v) = %q, %v", tt.channel, kind, tone)
		}
	}
}

func TestFilterChannelsMatchesNameAndType(t *testing.T) {
	channels := []panelChannel{{Name: "Main", Type: "anthropic"}, {Name: "Backup", Type: "openai"}}
	if got := filterChannels(channels, "OPENAI"); len(got) != 1 || got[0].Name != "Backup" {
		t.Fatalf("type match = %#v", got)
	}
	if got := filterChannels(channels, "mai"); len(got) != 1 || got[0].Name != "Main" {
		t.Fatalf("name match = %#v", got)
	}
}

func TestRunChannelTestSendsOneRequestForTheChannel(t *testing.T) {
	var posted []postedGraphQL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := readPostedGraphQL(r)
		if err != nil {
			t.Error(err)
		}
		posted = append(posted, call)
		_, _ = w.Write([]byte(`{"data":{"testChannel":{"latency":1.2345,"success":true,"error":null}}}`))
	}))
	defer server.Close()

	session := func() (desktopSession, error) { return desktopSession{Token: "t", ProjectID: "p"}, nil }
	got := runChannelTest(t.Context(), session, server.URL, "c1")
	if !got.OK || got.LatencyMs != 1235 || got.Running {
		t.Fatalf("result = %#v", got)
	}
	if len(posted) != 1 || posted[0].OperationName != "TestChannel" {
		t.Fatalf("posted = %#v", posted)
	}
	input, _ := json.Marshal(posted[0].Variables["input"])
	if string(input) != `{"channelID":"c1"}` {
		t.Fatalf("input = %s", input)
	}
}

func TestMapChannelTestKeepsTheProviderError(t *testing.T) {
	var data testChannelData
	if err := json.Unmarshal([]byte(`{"testChannel":{"latency":0.4,"success":false,"error":" 429 rate limited "}}`), &data); err != nil {
		t.Fatal(err)
	}
	if got := mapChannelTest(data); got.OK || got.Error != "429 rate limited" || got.LatencyMs != 400 {
		t.Fatalf("result = %#v", got)
	}
}
