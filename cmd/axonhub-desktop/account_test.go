package main

import (
	"context"
	"strings"
	"testing"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

type accountHarness struct {
	ctx     context.Context
	db      *ent.Client
	system  *biz.SystemService
	account desktopAccount
}

func newAccountHarness(t *testing.T) accountHarness {
	t.Helper()
	dsn := "file:desktop_account_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&_fk=1"
	db := enttest.NewEntClient(t, "sqlite3", dsn)
	t.Cleanup(func() { _ = db.Close() })
	cache := xcache.Config{Mode: xcache.ModeMemory}
	return accountHarness{
		ctx:     authz.WithTestBypass(ent.NewContext(t.Context(), db)),
		db:      db,
		system:  biz.NewSystemService(biz.SystemServiceParams{Ent: db, CacheConfig: cache}),
		account: desktopAccount{client: db, projects: biz.NewProjectService(biz.ProjectServiceParams{Ent: db, CacheConfig: cache})},
	}
}

func (h accountHarness) initialize(t *testing.T, email, password string) {
	t.Helper()
	if err := h.system.Initialize(h.ctx, &biz.InitializeSystemParams{
		OwnerEmail: email, OwnerPassword: password, OwnerFirstName: "Owner", OwnerLastName: "User", BrandName: brandName,
	}); err != nil {
		t.Fatal(err)
	}
}

func (h accountHarness) owner(t *testing.T) *ent.User {
	t.Helper()
	return h.db.User.Query().Where(user.EmailEQ(ownerEmail)).OnlyX(h.ctx)
}

func (h accountHarness) desktopProject(t *testing.T) *ent.Project {
	t.Helper()
	return h.db.Project.Query().Where(project.NameEQ(desktopProjectName)).OnlyX(h.ctx)
}

func (h accountHarness) isMember(t *testing.T, u *ent.User, p *ent.Project) bool {
	t.Helper()
	return h.db.UserProject.Query().
		Where(userproject.UserIDEQ(u.ID), userproject.ProjectIDEQ(p.ID), userproject.IsOwnerEQ(true)).
		ExistX(h.ctx)
}

func TestEnsureFreshInstallRenamesInitialProject(t *testing.T) {
	h := newAccountHarness(t)
	h.initialize(t, ownerEmail, "fresh-secret-0123")

	if err := h.account.ensure(t.Context(), "fresh-secret-0123", true); err != nil {
		t.Fatal(err)
	}
	if n := h.db.Project.Query().CountX(h.ctx); n != 1 {
		t.Fatalf("projects = %d, want the renamed initial project only", n)
	}
	owner := h.owner(t)
	if !owner.IsOwner || biz.VerifyPassword(owner.Password, "fresh-secret-0123") != nil {
		t.Fatalf("owner = %+v", owner)
	}
	if !h.isMember(t, owner, h.desktopProject(t)) {
		t.Fatal("the desktop owner must own the desktop project")
	}
}

func TestEnsureAdoptsAnotherInstallsDatabase(t *testing.T) {
	h := newAccountHarness(t)
	h.initialize(t, "me@example.com", "my-own-password")

	if err := h.account.ensure(t.Context(), "desktop-secret-1", false); err != nil {
		t.Fatal(err)
	}
	owner := h.owner(t)
	if !owner.IsOwner || owner.Status != user.StatusActivated || biz.VerifyPassword(owner.Password, "desktop-secret-1") != nil {
		t.Fatalf("desktop owner = %+v", owner)
	}
	if !h.isMember(t, owner, h.desktopProject(t)) {
		t.Fatal("the desktop owner must own the desktop project")
	}
	if !h.db.Project.Query().Where(project.NameEQ(initProjectName)).ExistX(h.ctx) {
		t.Fatal("the other install's Default project must stay as it is")
	}
	if !h.db.User.Query().Where(user.EmailEQ("me@example.com"), user.IsOwnerEQ(true)).ExistX(h.ctx) {
		t.Fatal("the other install's owner must stay an owner")
	}

	// A new secret on the next launch (owner.secret lost) resets the
	// password and creates nothing twice.
	if err := h.account.ensure(t.Context(), "desktop-secret-2", false); err != nil {
		t.Fatal(err)
	}
	if biz.VerifyPassword(h.owner(t).Password, "desktop-secret-2") != nil {
		t.Fatal("password was not reset to the new secret")
	}
	if n := h.db.Project.Query().Where(project.NameEQ(desktopProjectName)).CountX(h.ctx); n != 1 {
		t.Fatalf("desktop projects = %d", n)
	}
}

func TestEnsureRepairsExistingDesktopRecords(t *testing.T) {
	h := newAccountHarness(t)
	h.initialize(t, "me@example.com", "my-own-password")
	h.db.User.Create().SetEmail(ownerEmail).SetPassword("not-a-hash").
		SetIsOwner(false).SetStatus(user.StatusDeactivated).SaveX(h.ctx)
	archived := h.db.Project.Create().SetName(desktopProjectName).SetStatus(project.StatusArchived).SaveX(h.ctx)

	if err := h.account.ensure(t.Context(), "desktop-secret", false); err != nil {
		t.Fatal(err)
	}
	owner := h.owner(t)
	if !owner.IsOwner || owner.Status != user.StatusActivated || biz.VerifyPassword(owner.Password, "desktop-secret") != nil {
		t.Fatalf("owner = %+v", owner)
	}
	if got := h.desktopProject(t); got.ID != archived.ID || got.Status != project.StatusActive {
		t.Fatalf("project = %+v", got)
	}
	if !h.isMember(t, owner, archived) {
		t.Fatal("the desktop owner must join the existing desktop project")
	}
}
