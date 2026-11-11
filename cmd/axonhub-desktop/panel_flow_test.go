// 职责: 验证流向图的节点推导和尝试时间线的文案。
// 关联文件:
//   cmd/axonhub-desktop/panel_flow.go
// 遵循的 Rules:
//   .agents/rules/go-general.md

package main

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
)

func TestBuildFlowMergesCallersAndChannels(t *testing.T) {
	flow := buildFlow([]panelRequest{
		{ID: "1", Caller: "Cursor", Channel: "A", Status: statusProcessing, InProgress: true},
		{ID: "2", Caller: "Cursor", Channel: "B", Status: statusFailed},
		{ID: "3", Caller: "CLI", Channel: "A", Status: statusCompleted},
		{ID: "4", Caller: " ", Channel: ""},
	}, "2")
	if len(flow.Callers) != 2 || len(flow.Channels) != 2 {
		t.Fatalf("flow = %#v", flow)
	}
	cursor, a, b := flow.Callers[0], flow.Channels[0], flow.Channels[1]
	if cursor.Label != "Cursor" || !cursor.Active || !cursor.Selected {
		t.Fatalf("Cursor = %#v", cursor)
	}
	if a.Label != "A" || !a.Active || a.Selected || a.Tone != toneActive {
		t.Fatalf("A takes its newest request: %#v", a)
	}
	if b.Active || !b.Selected || b.Tone != toneDanger {
		t.Fatalf("B = %#v", b)
	}
	if !flow.active() {
		t.Fatal("a request in progress did not animate the flow")
	}
	if buildFlow(nil, "").active() {
		t.Fatal("an empty flow animates")
	}
}

func TestBuildFlowCapsEachSide(t *testing.T) {
	requests := lo.Times(flowNodeLimit+2, func(i int) panelRequest {
		return panelRequest{ID: fmt.Sprint(i), Caller: fmt.Sprintf("caller-%d", i), Channel: "one"}
	})
	flow := buildFlow(requests, "")
	if len(flow.Callers) != flowNodeLimit || flow.Callers[0].Label != "caller-0" {
		t.Fatalf("callers = %#v", flow.Callers)
	}
	if len(flow.Channels) != 1 {
		t.Fatalf("channels = %#v", flow.Channels)
	}
}

func TestCubicPointEnds(t *testing.T) {
	p0, p1, p2, p3 := [2]float32{0, 0}, [2]float32{5, 0}, [2]float32{5, 10}, [2]float32{10, 10}
	if x, y := cubicPoint(p0, p1, p2, p3, 0); x != 0 || y != 0 {
		t.Fatalf("t=0 → %v,%v", x, y)
	}
	if x, y := cubicPoint(p0, p1, p2, p3, 1); x != 10 || y != 10 {
		t.Fatalf("t=1 → %v,%v", x, y)
	}
	if x, y := cubicPoint(p0, p1, p2, p3, 0.5); x != 5 || y != 5 {
		t.Fatalf("t=0.5 → %v,%v", x, y)
	}
}

func TestPresentAttemptLeadsWithTheStatusCode(t *testing.T) {
	failed := presentAttempt(panelAttempt{Channel: "A", Status: statusFailed, StatusCode: lo.ToPtr(429), Error: "rate limited"})
	if failed.Result != "429" || failed.Tone != toneDanger || failed.Error != "rate limited" || failed.Metrics != "" {
		t.Fatalf("failed = %#v", failed)
	}
	running := presentAttempt(panelAttempt{Status: statusProcessing})
	if running.Channel != panelBlank || running.Tone != toneActive || running.Result != statusText(statusProcessing) {
		t.Fatalf("running = %#v", running)
	}
	done := presentAttempt(panelAttempt{Channel: "B", Status: statusCompleted, StatusCode: lo.ToPtr(200), LatencyMs: lo.ToPtr(1200), FirstTokenMs: lo.ToPtr(300)})
	if done.Tone != toneSuccess || done.Metrics == "" || done.Result != "200" {
		t.Fatalf("done = %#v", done)
	}
}

func TestAttemptErrorsNamesEachChannel(t *testing.T) {
	got := attemptErrors([]panelAttempt{
		{Channel: "A", Error: "429"},
		{Channel: "B"},
		{Error: "timeout"},
	})
	if got != "A: 429\n"+panelBlank+": timeout" {
		t.Fatalf("errors = %q", got)
	}
}

func TestPacketFliesThroughTheHubThenWaitsAtTheChannel(t *testing.T) {
	if at, alpha, ok := packetAt(0, true); !ok || at != 0 || alpha != 1 {
		t.Fatalf("start on the caller link = %v %v %v", at, alpha, ok)
	}
	if _, _, ok := packetAt(0, false); ok {
		t.Fatal("the packet is on both links at once")
	}
	if at, _, ok := packetAt(packetFly*0.75, false); !ok || at < 0.49 || at > 0.51 {
		t.Fatalf("midway on the channel link = %v %v", at, ok)
	}
	if _, _, ok := packetAt(packetFly*0.75, true); ok {
		t.Fatal("the packet went back to the caller link")
	}
	at, alpha, ok := packetAt(0.9, false)
	if !ok || at != 1 || alpha < 0.45 || alpha > 1 {
		t.Fatalf("waiting at the channel = %v %v %v", at, alpha, ok)
	}
	if _, _, ok := packetAt(0.9, true); ok {
		t.Fatal("a waiting packet is drawn on the caller link")
	}
}
