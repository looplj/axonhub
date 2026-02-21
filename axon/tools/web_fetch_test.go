package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchTool_Markdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>My Title</title><meta name="description" content="My Desc"></head><body><h1>Hello</h1><p>World</p></body></html>`))
	}))
	defer srv.Close()

	tool := NewWebFetchTool()

	res := tool.Execute(context.Background(), webFetchInput{Query: srv.URL})
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}

	if res.Content.Text == nil {
		t.Fatalf("expected text result")
	}

	out := *res.Content.Text
	if !strings.Contains(out, "# Hello") {
		t.Fatalf("expected markdown conversion, got: %s", out)
	}
}

func TestWebFetchTool_QueryRequired(t *testing.T) {
	tool := NewWebFetchTool()

	res := tool.Execute(context.Background(), webFetchInput{Query: ""})
	if res.Error == nil {
		t.Fatalf("expected error")
	}
}
