package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

func TestLanguageChoices(t *testing.T) {
	codes, names, chosen := languageChoices("")
	if len(codes) != 3 || names[0] != locales.FollowSystem() || chosen != names[0] {
		t.Fatalf("system = %q %q %q", codes, names, chosen)
	}
	if names[1] != i18n.Autonym("en") || names[2] == "" || names[2] == "zh-Hans" {
		t.Fatalf("autonyms = %q", names)
	}
	_, _, chosen = languageChoices("zh-Hans")
	if chosen != names[2] {
		t.Fatalf("chinese choice = %q, want %q", chosen, names[2])
	}
	_, _, chosen = languageChoices("en")
	if chosen != names[1] {
		t.Fatalf("english choice = %q, want %q", chosen, names[1])
	}
}

func TestRelativePast(t *testing.T) {
	now := time.Now()
	if relativePast(now) == "" || relativePast(now.Add(-3*time.Minute)) == "" || relativePast(now.Add(-48*time.Hour)) == "" {
		t.Fatal("a relative time was empty")
	}
}
