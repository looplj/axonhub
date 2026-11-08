package main

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

func TestRequestAdminPathEscapesGID(t *testing.T) {
	const id = "gid://axonhub/Request/1"
	got := requestAdminPath(id)
	want := "/project/requests/" + url.PathEscape(id)
	if got != want {
		t.Fatalf("requestAdminPath() = %q, want %q", got, want)
	}
	if got != "/project/requests/gid:%2F%2Faxonhub%2FRequest%2F1" {
		t.Fatalf("requestAdminPath() = %q", got)
	}
	if strings.Contains(got, "://") {
		t.Fatalf("requestAdminPath() = %q", got)
	}
}

func TestCardAnimatesOnlyWhileSelectedRequestIsInProgress(t *testing.T) {
	requests := []panelRequest{
		{ID: "done", Status: statusCompleted},
		{ID: "live", Status: statusProcessing, InProgress: true},
		{ID: "wait", Status: statusPending, InProgress: true},
		{ID: "bad", Status: statusFailed},
		{ID: "stop", Status: statusCanceled},
	}
	tests := []struct {
		id   string
		want bool
	}{
		{id: "done", want: false},
		{id: "live", want: true},
		{id: "wait", want: true},
		{id: "bad", want: false},
		{id: "stop", want: false},
		{id: "", want: false},
		{id: "missing", want: false},
	}
	for _, tt := range tests {
		if got := cardAnimates(requests, tt.id); got != tt.want {
			t.Fatalf("cardAnimates(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}

	selected := nextSelectedID(requests, "")
	if selected != "done" {
		t.Fatalf("first loaded row = %q", selected)
	}
	if cardAnimates(requests, selected) {
		t.Fatal("the completed first row animates the wires")
	}
	if !cardAnimates(requests, nextSelectedID(requests, "live")) {
		t.Fatal("the selected in-progress row does not animate the wires")
	}
	if nextSelectedID(requests, "gone") != "done" {
		t.Fatal("a missing selection did not fall back to the first row")
	}
	if nextSelectedID(nil, "live") != "" {
		t.Fatal("an empty list kept a selection")
	}
	if cardAnimates([]panelRequest{{ID: "x", Status: statusProcessing}}, "x") {
		t.Fatal("a processing status animates without InProgress")
	}
	if !cardAnimates([]panelRequest{{ID: "x", InProgress: true}}, "x") {
		t.Fatal("InProgress does not animate the selected row")
	}
}

func TestPanelStats(t *testing.T) {
	got := panelStats(panelSnapshot{
		RequestsToday: 1284,
		InputTokens:   1000,
		OutputTokens:  250,
		CachedTokens:  80,
		Failures:      3,
	})
	want := []panelStat{
		{Label: locales.Today(), Value: "1.3k"},
		{Label: locales.Tokens(), Value: "1.3k"},
		{Label: locales.Cached(), Value: "80"},
		{Label: locales.Failed(), Value: "3", Tone: toneDanger},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stats = %+v", got)
	}
	if quiet := panelStats(panelSnapshot{}); quiet[3].Tone != toneMuted || quiet[3].Value != "0" {
		t.Fatalf("no failures = %+v", quiet[3])
	}
}

func TestPresentRequest(t *testing.T) {
	if panelLabel("  ") != "—" || panelLabel("Dart") != "Dart" {
		t.Fatal("panelLabel")
	}

	completed := presentRequest(panelRequest{
		Caller:          "",
		Channel:         "openai",
		Status:          statusCompleted,
		TotalTokens:     lo.ToPtr(88900),
		LatencyMs:       lo.ToPtr(6300),
		TokensPerSecond: lo.ToPtr(170),
	})
	want := requestResult{Channel: "openai", Primary: locales.TokenCount("88.9k"), Secondary: locales.Seconds("6.3") + " · " + locales.TokensPerSecond("170"), Tone: toneSuccess}
	if completed != want {
		t.Fatalf("completed = %+v", completed)
	}

	noTokens := presentRequest(panelRequest{Status: statusCompleted, LatencyMs: lo.ToPtr(640)})
	if noTokens.Primary != locales.Milliseconds("640") || noTokens.Secondary != "" {
		t.Fatalf("completed without tokens = %+v", noTokens)
	}

	failed := presentRequest(panelRequest{
		Channel:         "fail-channel",
		Status:          statusFailed,
		TotalTokens:     lo.ToPtr(60),
		LatencyMs:       lo.ToPtr(500),
		TokensPerSecond: lo.ToPtr(99),
	})
	if failed.Tone != toneDanger || failed.Primary != statusText(statusFailed) || failed.Secondary != locales.TokenCount("60")+" · "+locales.Milliseconds("500") {
		t.Fatalf("failed = %+v", failed)
	}

	canceled := presentRequest(panelRequest{
		Status:          statusCanceled,
		LatencyMs:       lo.ToPtr(1800),
		TokensPerSecond: lo.ToPtr(12),
	})
	if canceled.Tone != toneDanger || canceled.Primary != statusText(statusCanceled) || canceled.Channel != "—" || canceled.Secondary != locales.Seconds("1.8") {
		t.Fatalf("canceled = %+v", canceled)
	}

	live := presentRequest(panelRequest{
		Caller:          "live",
		Status:          statusProcessing,
		InProgress:      true,
		TotalTokens:     lo.ToPtr(16),
		LatencyMs:       lo.ToPtr(100),
		TokensPerSecond: lo.ToPtr(8),
	})
	if live.Tone != toneActive || live.Primary != statusText(statusProcessing) || live.Secondary != "" || live.Channel != "—" {
		t.Fatalf("in progress = %+v", live)
	}
}

func TestRoutingStatusAndSummary(t *testing.T) {
	done := panelRequest{Status: statusCompleted, TotalTokens: lo.ToPtr(12400), LatencyMs: lo.ToPtr(2800), TokensPerSecond: lo.ToPtr(86)}
	tests := []struct {
		name     string
		request  panelRequest
		selected bool
		label    string
		tone     statusTone
	}{
		{"none", panelRequest{}, false, locales.Waiting(), toneMuted},
		{"live", panelRequest{Status: statusProcessing, InProgress: true}, true, locales.InProgress(), toneActive},
		{"completed", done, true, locales.Completed(), toneSuccess},
		{"failed", panelRequest{Status: statusFailed}, true, locales.Failed(), toneDanger},
		{"canceled", panelRequest{Status: statusCanceled}, true, locales.Canceled(), toneDanger},
	}
	for _, tt := range tests {
		label, tone := routingStatus(tt.request, tt.selected)
		if label != tt.label || tone != tt.tone {
			t.Fatalf("%s: routingStatus = %q, %v", tt.name, label, tone)
		}
	}
	if got, want := routingSummary(done), locales.TokenCount("12.4k")+" · "+locales.Seconds("2.8")+" · "+locales.TokensPerSecond("86"); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if got := routingSummary(panelRequest{InProgress: true, LatencyMs: lo.ToPtr(5)}); got != locales.InProgressLower() {
		t.Fatalf("live summary = %q", got)
	}
}
