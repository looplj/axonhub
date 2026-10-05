package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// newAllowedIPsTestCase creates a service plus the user/project pair needed to
// create API keys.
func newAllowedIPsTestCase(t *testing.T) (*APIKeyService, *ent.Client, context.Context, int) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	projectService := &ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](xcache.Config{Mode: xcache.ModeMemory}),
	}

	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		CacheConfig:    xcache.Config{Mode: xcache.ModeMemory},
		Ent:            client,
		ProjectService: projectService,
		KeyPrefix:      "ah",
	})
	t.Cleanup(apiKeyService.Stop)

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("allowed-ips-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	name := fmt.Sprintf("project-%d", time.Now().UnixNano())
	testProject, err := client.Project.Create().
		SetName(name).
		SetDescription(name).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	return apiKeyService, client, contexts.WithUser(ctx, testUser), testProject.ID
}

// TestUpdateAPIKey_EmptyAllowedIPsLeavesListUntouched pins the behaviour the UI
// has to work around: an empty list is "leave unchanged", not "clear".
func TestUpdateAPIKey_EmptyAllowedIPsLeavesListUntouched(t *testing.T) {
	service, client, ctx, projectID := newAllowedIPsTestCase(t)

	created, err := service.CreateAPIKey(ctx, ent.CreateAPIKeyInput{
		Name:       "restricted",
		ProjectID:  projectID,
		AllowedIps: []string{"1.2.3.4"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"1.2.3.4"}, created.AllowedIps)

	updated, err := service.UpdateAPIKey(ctx, created.ID, ent.UpdateAPIKeyInput{AllowedIps: []string{}})
	require.NoError(t, err)
	require.Equal(t, []string{"1.2.3.4"}, updated.AllowedIps,
		"an empty list must not clear the allowlist")

	reloaded, err := client.APIKey.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"1.2.3.4"}, reloaded.AllowedIps)
}

// TestUpdateAPIKey_ClearAllowedIPsEmptiesList covers the flag the UI now sends
// when the IP restriction switch is turned off.
func TestUpdateAPIKey_ClearAllowedIPsEmptiesList(t *testing.T) {
	service, client, ctx, projectID := newAllowedIPsTestCase(t)

	created, err := service.CreateAPIKey(ctx, ent.CreateAPIKeyInput{
		Name:       "restricted",
		ProjectID:  projectID,
		AllowedIps: []string{"1.2.3.4", "10.0.0.0/8"},
	})
	require.NoError(t, err)

	updated, err := service.UpdateAPIKey(ctx, created.ID, ent.UpdateAPIKeyInput{ClearAllowedIps: true})
	require.NoError(t, err)
	require.Empty(t, updated.AllowedIps)

	reloaded, err := client.APIKey.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Empty(t, reloaded.AllowedIps, "the stored allowlist must be cleared")
}
