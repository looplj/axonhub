// 职责: 验证渠道和配额提醒的判定：基线不报、持续状态只报一次、恢复后可再报，超出的留到下一次。
// 关联文件:
//   cmd/axonhub-desktop/panel_watch.go
// 遵循的 Rules:
//   .agents/rules/go-general.md

package main

import (
	"path/filepath"
	"testing"
)

func TestWatchFirstCheckIsTheBaseline(t *testing.T) {
	var w watchState
	down := []panelChannel{{ID: "c1", Name: "Main", AutoDisabled: true}, {ID: "c2", Name: "Q", Enabled: true, Quota: quotaExhausted}}
	if got := w.alerts(down); len(got) != 0 {
		t.Fatalf("baseline alerted: %#v", got)
	}
	if got := w.alerts(down); len(got) != 0 {
		t.Fatalf("a lasting condition alerted again: %#v", got)
	}
}

func TestWatchReportsNewConditionsOnce(t *testing.T) {
	var w watchState
	w.alerts([]panelChannel{{ID: "c1", Name: "Main", Enabled: true}})

	got := w.alerts([]panelChannel{{ID: "c1", Name: "Main", AutoDisabled: true}, {ID: "c2", Name: "Q", Enabled: true, Quota: quotaExhausted}})
	want := []watchAlert{{Kind: alertChannel, ID: "c1", Name: "Main"}, {Kind: alertQuota, ID: "c2", Name: "Q"}}
	if len(got) != len(want) {
		t.Fatalf("alerts = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("alerts[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}

	if got := w.alerts([]panelChannel{{ID: "c1", Name: "Main", Enabled: true}}); len(got) != 0 {
		t.Fatalf("recovery alerted: %#v", got)
	}
	if got := w.alerts([]panelChannel{{ID: "c1", Name: "Main", AutoDisabled: true}}); len(got) != 1 || got[0].Kind != alertChannel {
		t.Fatalf("a channel that went down again was not reported: %#v", got)
	}
}

func TestWatchIgnoresOperatorDisables(t *testing.T) {
	var w watchState
	w.alerts(nil)
	if got := w.alerts([]panelChannel{{ID: "c1", Name: "Off"}, {ID: "c2", Name: "Off quota", Quota: quotaExhausted}}); len(got) != 0 {
		t.Fatalf("a manual disable alerted: %#v", got)
	}
}

func TestWatchKeepsOverflowForTheNextCheck(t *testing.T) {
	var w watchState
	w.alerts(nil)
	channels := []panelChannel{
		{ID: "c1", Name: "A", AutoDisabled: true},
		{ID: "c2", Name: "B", AutoDisabled: true},
		{ID: "c3", Name: "C", AutoDisabled: true},
		{ID: "c4", Name: "D", AutoDisabled: true},
	}
	got := w.alerts(channels)
	if len(got) != watchAlertsMax || got[0].Name != "A" || got[2].Name != "C" {
		t.Fatalf("first alerts = %#v", got)
	}
	again := w.alerts(channels)
	if len(again) != 1 || again[0].ID != "c4" {
		t.Fatalf("overflow = %#v", again)
	}
	if more := w.alerts(channels); len(more) != 0 {
		t.Fatalf("reported again: %#v", more)
	}
}

func TestWatchDropsPendingAlertAfterRecovery(t *testing.T) {
	var w watchState
	w.alerts(nil)
	w.alerts([]panelChannel{
		{ID: "c1", Name: "A", AutoDisabled: true},
		{ID: "c2", Name: "B", AutoDisabled: true},
		{ID: "c3", Name: "C", AutoDisabled: true},
		{ID: "c4", Name: "D", AutoDisabled: true},
	})
	if got := w.alerts([]panelChannel{
		{ID: "c1", Name: "A", AutoDisabled: true},
		{ID: "c2", Name: "B", AutoDisabled: true},
		{ID: "c3", Name: "C", AutoDisabled: true},
	}); len(got) != 0 {
		t.Fatalf("recovered overflow alerted: %#v", got)
	}
}

func TestNotifyPrefIsOffByDefaultAndSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), panelPrefsName)
	if readPanelPrefs(path).Notify {
		t.Fatal("alerts are on without a saved choice")
	}
	if err := writePanelPrefs(path, panelPrefs{Notify: true}); err != nil {
		t.Fatal(err)
	}
	if !readPanelPrefs(path).Notify {
		t.Fatal("the saved choice was not read back")
	}
}
