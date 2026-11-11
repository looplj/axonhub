package main

import (
	"strings"
	"testing"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestArchivedDesktopKeyStaysReservedUntilRestored(t *testing.T) {
	h := newAccountHarness(t)
	h.initialize(t, ownerEmail, "desktop-secret-0123")
	if err := h.account.ensure(t.Context(), "desktop-secret-0123", true); err != nil {
		t.Fatal(err)
	}
	keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig:    xcache.Config{Mode: xcache.ModeMemory},
		Ent:            h.db,
		ProjectService: h.account.projects,
		KeyPrefix:      "ah",
	})
	t.Cleanup(keys.Stop)

	ctx := contexts.WithUser(h.ctx, h.owner(t))
	projectID := h.desktopProject(t).ID
	created, err := keys.CreateAPIKey(ctx, ent.CreateAPIKeyInput{
		Name:      desktopKeyName,
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keys.UpdateAPIKeyStatus(ctx, created.ID, apikey.StatusArchived); err != nil {
		t.Fatal(err)
	}

	_, err = keys.CreateAPIKey(ctx, ent.CreateAPIKeyInput{
		Name:      desktopKeyName,
		ProjectID: projectID,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("creating another key named %q while one is archived: %v", desktopKeyName, err)
	}

	restored, err := keys.UpdateAPIKeyStatus(ctx, created.ID, apikey.StatusEnabled)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != created.ID || restored.Key != created.Key || restored.Status != apikey.StatusEnabled {
		t.Fatalf("restored key = id %d status %s", restored.ID, restored.Status)
	}
}
