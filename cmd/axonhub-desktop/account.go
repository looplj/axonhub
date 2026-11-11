package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/server/biz"
)

// initProjectName is the project system initialization creates. A fresh
// desktop install renames it to desktopProjectName instead of keeping an
// empty second project.
const initProjectName = "Default"

// desktopAccount makes the desktop owner and its project ready in the
// database the server runs on. The desktop process runs it in process,
// after the server answers and before it signs in.
type desktopAccount struct {
	client   *ent.Client
	projects *biz.ProjectService
}

// ensure makes desktop@axonhub.local a system owner whose password is
// secret, and gives it the AxonHub Desktop project. It works on a database
// another AxonHub already initialized: the account is created or its
// password reset, and the project created when missing. fresh reports that
// this launch initialized the system, so its Default project becomes the
// desktop project. Everything runs in one transaction.
func (a desktopAccount) ensure(ctx context.Context, secret string, fresh bool) (err error) {
	ctx = authz.WithSystemBypass(ent.NewContext(ctx, a.client), "desktop-account")
	tx, err := a.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("start desktop account transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	ctx = ent.NewContext(ent.NewTxContext(ctx, tx), tx.Client())

	owner, err := ensureDesktopOwner(ctx, tx.Client(), secret)
	if err != nil {
		return err
	}
	if err := a.ensureDesktopProject(contexts.WithUser(ctx, owner), tx.Client(), owner, fresh); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit desktop account: %w", err)
	}
	return nil
}

func ensureDesktopOwner(ctx context.Context, client *ent.Client, secret string) (*ent.User, error) {
	existing, err := client.User.Query().Where(user.EmailEQ(ownerEmail)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("load desktop owner: %w", err)
	}
	if existing == nil {
		hashed, err := biz.HashPassword(secret)
		if err != nil {
			return nil, errors.New("hash desktop owner password")
		}
		created, err := client.User.Create().
			SetEmail(ownerEmail).
			SetPassword(hashed).
			SetFirstName(ownerFirstName).
			SetLastName(ownerLastName).
			SetIsOwner(true).
			SetScopes([]string{"*"}).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("create desktop owner: %w", err)
		}
		return created, nil
	}

	update := existing.Update()
	changed := false
	if biz.VerifyPassword(existing.Password, secret) != nil {
		// The database came from another install, or owner.secret was
		// lost: this install's secret becomes the password.
		hashed, err := biz.HashPassword(secret)
		if err != nil {
			return nil, errors.New("hash desktop owner password")
		}
		update.SetPassword(hashed)
		changed = true
	}
	if !existing.IsOwner {
		update.SetIsOwner(true)
		changed = true
	}
	if existing.Status != user.StatusActivated {
		update.SetStatus(user.StatusActivated)
		changed = true
	}
	if !lo.Contains(existing.Scopes, "*") {
		update.SetScopes([]string{"*"})
		changed = true
	}
	if !changed {
		return existing, nil
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update desktop owner: %w", err)
	}
	return updated, nil
}

func (a desktopAccount) ensureDesktopProject(ctx context.Context, client *ent.Client, owner *ent.User, fresh bool) error {
	existing, err := client.Project.Query().Where(project.NameEQ(desktopProjectName)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("load desktop project: %w", err)
	}
	if existing == nil && fresh {
		existing, err = client.Project.Query().Where(project.NameEQ(initProjectName)).Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("load initial project: %w", err)
		}
		if existing != nil {
			existing, err = existing.Update().SetName(desktopProjectName).Save(ctx)
			if err != nil {
				return fmt.Errorf("rename initial project: %w", err)
			}
		}
	}
	if existing == nil {
		// CreateProject adds the project's roles and makes owner its owner.
		if _, err := a.projects.CreateProject(ctx, ent.CreateProjectInput{
			Name:        desktopProjectName,
			Description: lo.ToPtr("Requests from AxonHub Desktop"),
		}); err != nil {
			return fmt.Errorf("create desktop project: %w", err)
		}
		return nil
	}

	if existing.Status != project.StatusActive {
		if _, err := existing.Update().SetStatus(project.StatusActive).Save(ctx); err != nil {
			return fmt.Errorf("activate desktop project: %w", err)
		}
	}
	member, err := client.UserProject.Query().
		Where(userproject.UserIDEQ(owner.ID), userproject.ProjectIDEQ(existing.ID)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("load desktop project membership: %w", err)
	}
	if member {
		return nil
	}
	if _, err := client.UserProject.Create().
		SetUserID(owner.ID).
		SetProjectID(existing.ID).
		SetIsOwner(true).
		Save(ctx); err != nil {
		return fmt.Errorf("join desktop project: %w", err)
	}
	return nil
}
