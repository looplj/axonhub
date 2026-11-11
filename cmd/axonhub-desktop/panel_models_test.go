// 职责: 验证 Models 页的数据层：读取 Model 实体、搜索和摘要行。
// 关联文件:
//   cmd/axonhub-desktop/panel_models.go
// 遵循的 Rules:
//   .agents/rules/go-general.md

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadPanelModelsReadsModelEntities(t *testing.T) {
	var posted postedGraphQL
	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("X-Project-ID")
		posted, _ = readPostedGraphQL(r)
		_, _ = w.Write([]byte(`{"data":{"models":{"edges":[
			{"node":{"id":"m1","modelID":"claude-x","name":"Claude X","developer":"anthropic","type":"chat","status":"enabled","associatedChannelCount":2}},
			{"node":null},
			{"node":{"id":"m2","modelID":"embed-1","name":"embed-1","developer":"","type":"embedding","status":"disabled","associatedChannelCount":0}}
		]}}}`))
	}))
	defer server.Close()

	models, err := loadPanelModels(t.Context(), server.Client(), server.URL+"/", "t")
	if err != nil {
		t.Fatal(err)
	}
	if posted.OperationName != "DesktopPanelModels" || header != "" {
		t.Fatalf("operation = %q, X-Project-ID = %q", posted.OperationName, header)
	}
	want := []panelModel{
		{ID: "m1", ModelID: "claude-x", Name: "Claude X", Developer: "anthropic", Type: "chat", Enabled: true, Channels: 2},
		{ID: "m2", ModelID: "embed-1", Name: "embed-1", Type: "embedding"},
	}
	if len(models) != 2 || models[0] != want[0] || models[1] != want[1] {
		t.Fatalf("models = %#v", models)
	}
}

func TestLoadPanelModelsReadsTheNextPage(t *testing.T) {
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted, err := readPostedGraphQL(r)
		if err != nil {
			t.Error(err)
		}
		after, _ := posted.Variables["after"].(string)
		cursors = append(cursors, after)
		if after == "" {
			_, _ = w.Write([]byte(`{"data":{"models":{"edges":[{"node":{"id":"m1","modelID":"a","name":"A","status":"enabled","associatedChannelCount":1}}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"models":{"edges":[{"node":{"id":"m2","modelID":"b","name":"B","status":"disabled","associatedChannelCount":0}}],"pageInfo":{"hasNextPage":false}}}}`))
	}))
	defer server.Close()

	models, err := loadPanelModels(t.Context(), server.Client(), server.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ModelID != "a" || models[1].ModelID != "b" || models[1].Enabled {
		t.Fatalf("models = %#v", models)
	}
	if len(cursors) != 2 || cursors[1] != "c1" {
		t.Fatalf("cursors = %#v", cursors)
	}
}

func TestFilterModelsMatchesIDNameAndDeveloper(t *testing.T) {
	models := []panelModel{
		{ModelID: "claude-x", Name: "Claude X", Developer: "anthropic"},
		{ModelID: "gpt-y", Name: "GPT Y", Developer: "openai"},
	}
	for query, want := range map[string]string{"CLAUDE": "claude-x", "gpt y": "gpt-y", "openai": "gpt-y"} {
		if got := filterModels(models, query); len(got) != 1 || got[0].ModelID != want {
			t.Errorf("filterModels(%q) = %#v", query, got)
		}
	}
	if got := filterModels(models, " "); len(got) != 2 {
		t.Fatalf("blank query kept %d", len(got))
	}
}

func TestModelSummaryOmitsANameThatRepeatsTheID(t *testing.T) {
	if got := modelSummary(panelModel{ModelID: "claude-x", Name: "Claude X", Developer: "anthropic"}, "2 channels"); got != "Claude X · anthropic · 2 channels" {
		t.Fatalf("summary = %q", got)
	}
	if got := modelSummary(panelModel{ModelID: "embed-1", Name: "embed-1"}, "0 channels"); got != "0 channels" {
		t.Fatalf("summary = %q", got)
	}
}
