package memory

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Model() string { return "fake-embedding" }

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	vectors := make([][]float64, 0, len(texts))
	for _, text := range texts {
		normalized := strings.ToLower(text)
		switch {
		case strings.Contains(normalized, "terse updates"), strings.Contains(normalized, "brief status"):
			vectors = append(vectors, []float64{1, 0})
		case strings.Contains(normalized, "quota"), strings.Contains(normalized, "429"):
			vectors = append(vectors, []float64{0, 1})
		default:
			vectors = append(vectors, []float64{0.2, 0.2})
		}
	}
	return vectors, nil
}

type hookStub struct {
	responses map[string][]json.RawMessage
}

func (h hookStub) CallHook(_ context.Context, hook string, _ any) ([]json.RawMessage, error) {
	return h.responses[hook], nil
}

func TestStoreSearchUsesSemanticEmbeddingsForLongTermMemory(t *testing.T) {
	layout := NewLayout(t.TempDir())
	store := NewStore(StoreOptions{
		Layout:   layout,
		Embedder: fakeEmbedder{},
		Now: func() time.Time {
			return time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)
		},
	})

	if err := writeMemoryFile(layout.LongTermPath(), "## Preferences\n\nUser prefers terse updates.\n"); err != nil {
		t.Fatalf("writeMemoryFile() error = %v", err)
	}

	matches, err := store.Search(context.Background(), "brief status messages", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("Search() returned no matches")
	}

	if got := matches[0].Text; !strings.Contains(got, "terse updates") {
		t.Fatalf("top match = %q, want terse updates memory", got)
	}

	if matches[0].Reason != "semantic" {
		t.Fatalf("top match reason = %q, want semantic", matches[0].Reason)
	}

	if matches[0].Label != filepath.ToSlash(filepath.Join(".axonclaw", "MEMORY.md")) {
		t.Fatalf("top match label = %q", matches[0].Label)
	}
}

func TestStoreAddCanBeExtendedByPluginHooks(t *testing.T) {
	layout := NewLayout(t.TempDir())
	payload, err := json.Marshal(MemoryAddHookOutput{
		ExtraEntries: []PluginMemoryEntry{{
			Target:  "longterm",
			Content: "Remember to keep rollout notes durable.",
		}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	store := NewStore(StoreOptions{
		Layout: layout,
		Hooks: hookStub{responses: map[string][]json.RawMessage{
			"memory.add": {payload},
		}},
		Now: func() time.Time {
			return time.Date(2026, 3, 17, 11, 0, 0, 0, time.UTC)
		},
	})

	if _, _, err := store.Add(context.Background(), "Temporary work note", false); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	content, err := readMemoryFile(layout.LongTermPath())
	if err != nil {
		t.Fatalf("readMemoryFile() error = %v", err)
	}

	if !strings.Contains(content, "Remember to keep rollout notes durable.") {
		t.Fatalf("long-term memory = %q, want plugin-promoted entry", content)
	}
}

func TestStoreSearchCanMergePluginResults(t *testing.T) {
	layout := NewLayout(t.TempDir())
	payload, err := json.Marshal(MemorySearchHookOutput{
		Results: []SearchMatch{{
			Label:  ".axonclaw/plugins/recalled.md",
			Line:   1,
			Text:   "Plugin-sourced memory",
			Score:  0.99,
			Reason: "plugin",
			Source: "plugin",
		}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	store := NewStore(StoreOptions{
		Layout: layout,
		Hooks: hookStub{responses: map[string][]json.RawMessage{
			"memory.search": {payload},
		}},
	})

	matches, err := store.Search(context.Background(), "anything", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(matches) != 1 || matches[0].Reason != "plugin" {
		t.Fatalf("matches = %#v, want plugin result", matches)
	}
}
