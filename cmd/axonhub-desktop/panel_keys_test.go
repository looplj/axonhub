// 职责: 验证 Keys 页的数据层：跨项目读取、打码、搜索、分组和状态变量。
// 关联文件:
//   cmd/axonhub-desktop/panel_keys.go
//   cmd/axonhub-desktop/panel_load_test.go
// 遵循的 Rules:
//   .agents/rules/go-general.md

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadPanelKeysReadsEveryProject(t *testing.T) {
	var header string
	var posted graphQLRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("X-Project-ID")
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"data":{"apiKeys":{"edges":[
			{"node":{"id":"k1","name":"CI","key":"ah-0123456789abcdef","type":"service_account","status":"disabled","project":{"id":"p2","name":"Team"}}},
			{"node":null},
			{"node":{"id":"k2","name":"AxonHub Desktop","key":"ah-fedcba9876543210","type":"user","status":"enabled","project":{"id":"p1","name":"AxonHub Desktop"}}}
		]}}}`))
	}))
	defer server.Close()

	keys, err := loadPanelKeys(t.Context(), server.Client(), server.URL+"/", "desktop-token")
	if err != nil {
		t.Fatal(err)
	}
	if header != "" {
		t.Fatalf("X-Project-ID = %q, want none so every project is read", header)
	}
	if posted.OperationName != "DesktopPanelKeys" {
		t.Fatalf("operation = %q", posted.OperationName)
	}
	want := []panelKey{
		{ID: "k1", Name: "CI", Secret: "ah-0123456789abcdef", Type: "service_account", ProjectID: "p2", ProjectName: "Team"},
		{ID: "k2", Name: "AxonHub Desktop", Secret: "ah-fedcba9876543210", Type: "user", Enabled: true, ProjectID: "p1", ProjectName: "AxonHub Desktop"},
	}
	if len(keys) != len(want) {
		t.Fatalf("keys = %#v", keys)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys[%d] = %#v, want %#v", i, keys[i], want[i])
		}
	}
}

func TestLoadPanelKeysErrorOmitsSecret(t *testing.T) {
	const secret = "ah-super-secret-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"boom ` + secret + `"}]}`))
	}))
	defer server.Close()

	_, err := loadPanelKeys(t.Context(), server.Client(), server.URL, "desktop-token")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
}

func TestMaskKey(t *testing.T) {
	tests := map[string]string{
		"":                    keyMaskDots,
		"short-key":           keyMaskDots,
		"ah-0123456789abcdef": "ah-012" + keyMaskDots + "cdef",
	}
	for secret, want := range tests {
		if got := maskKey(secret); got != want {
			t.Errorf("maskKey(%q) = %q, want %q", secret, got, want)
		}
	}
}

func TestFilterKeysMatchesNameAndProject(t *testing.T) {
	keys := []panelKey{
		{ID: "1", Name: "Cursor", ProjectName: "Work"},
		{ID: "2", Name: "CI", ProjectName: "Team"},
		{ID: "3", Name: "Laptop", ProjectName: "Work"},
	}
	if got := filterKeys(keys, "  "); len(got) != 3 {
		t.Fatalf("blank query kept %d", len(got))
	}
	if got := filterKeys(keys, "cur"); len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("name match = %#v", got)
	}
	if got := filterKeys(keys, "WORK"); len(got) != 2 {
		t.Fatalf("project match = %#v", got)
	}
}

func TestGroupKeysLeadsWithDesktopProject(t *testing.T) {
	keys := []panelKey{
		{ID: "1", ProjectID: "a"},
		{ID: "2", ProjectID: "b"},
		{ID: "3", ProjectID: "desktop"},
		{ID: "4", ProjectID: "a"},
	}
	groups := groupKeys(keys, "desktop")
	var order []string
	for _, g := range groups {
		order = append(order, g.ProjectID)
	}
	if strings.Join(order, ",") != "desktop,a,b" {
		t.Fatalf("order = %v", order)
	}
	if len(groups[1].Keys) != 2 || groups[1].Keys[1].ID != "4" {
		t.Fatalf("project a keys = %#v", groups[1].Keys)
	}
	if got := groupKeys(keys, "missing"); got[0].ProjectID != "a" {
		t.Fatalf("unknown first project reordered groups: %#v", got)
	}
}

func TestLoadPanelKeysReadsTheNextPage(t *testing.T) {
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted, err := readPostedGraphQL(r)
		if err != nil {
			t.Error(err)
		}
		after, _ := posted.Variables["after"].(string)
		cursors = append(cursors, after)
		if after == "" {
			_, _ = w.Write([]byte(`{"data":{"apiKeys":{"edges":[{"node":{"id":"k1","name":"A","key":"ah-0123456789abcdef","status":"enabled","project":{"id":"p","name":"P"}}}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"apiKeys":{"edges":[{"node":{"id":"k2","name":"B","key":"ah-fedcba9876543210","status":"disabled","project":{"id":"p","name":"P"}}}],"pageInfo":{"hasNextPage":false}}}}`))
	}))
	defer server.Close()

	keys, err := loadPanelKeys(t.Context(), server.Client(), server.URL, "desktop-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].ID != "k1" || keys[1].ID != "k2" {
		t.Fatalf("keys = %#v", keys)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "c1" {
		t.Fatalf("cursors = %#v", cursors)
	}
}

func TestMergeBusyKeysKeepsTheSwitch(t *testing.T) {
	current := []panelKey{{ID: "k1", Enabled: true}, {ID: "k2", Enabled: false}}
	incoming := []panelKey{{ID: "k1", Enabled: false, Name: "A"}, {ID: "k2", Enabled: true, Name: "B"}}
	got := mergeBusyKeys(incoming, current, map[string]bool{"k1": true})
	if !got[0].Enabled || got[0].Name != "A" || !got[1].Enabled {
		t.Fatalf("merged = %#v", got)
	}
	if untouched := mergeBusyKeys(incoming, current, nil); untouched[0].Enabled {
		t.Fatalf("an idle key was kept: %#v", untouched[0])
	}
}

func TestKeyStatusVariables(t *testing.T) {
	if v := keyStatusVariables("k1", false); v["id"] != "k1" || v["status"] != statusKeyDisabled {
		t.Fatalf("disable = %#v", v)
	}
	if v := keyStatusVariables("k1", true); v["status"] != statusKeyEnabled {
		t.Fatalf("enable = %#v", v)
	}
}
