package datamigrate_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/agenthost"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
)

func TestV1_0_0_CreateLocalAgentHost(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	err := datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	host, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, "Local", host.Name)
	assert.Equal(t, agenthost.TypeLocal, host.Type)
	assert.Equal(t, agenthost.StatusActive, host.Status)
	assert.Equal(t, filepath.Join(homeDir, ".axonclaw", "hosts"), host.Directory)
}

func TestV1_0_0_LocalAgentHostAlreadyExists(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	existingHost, err := client.AgentHost.Create().
		SetName("existing-local").
		SetType(agenthost.TypeLocal).
		SetStatus(agenthost.StatusActive).
		SetDirectory("/tmp/existing-local").
		Save(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	count, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	host, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, existingHost.ID, host.ID)
	assert.Equal(t, "existing-local", host.Name)
	assert.Equal(t, "/tmp/existing-local", host.Directory)
}

func TestV1_0_0_Idempotency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	err := datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	host1, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	count, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	host2, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, host1.ID, host2.ID)
}

func TestV1_0_0_VerifyAgentHostFields(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	err := datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	host, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.NotZero(t, host.ID)
	assert.NotZero(t, host.CreatedAt)
	assert.NotZero(t, host.UpdatedAt)
	assert.Equal(t, "Local", host.Name)
	assert.Equal(t, agenthost.TypeLocal, host.Type)
	assert.Equal(t, agenthost.StatusActive, host.Status)
	assert.Equal(t, filepath.Join(homeDir, ".axonclaw", "hosts"), host.Directory)
}

func TestV1_0_0_MultipleNonLocalAgentHosts(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	_, err := client.AgentHost.Create().
		SetName("vm-host").
		SetType(agenthost.TypeVM).
		SetStatus(agenthost.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.AgentHost.Create().
		SetName("docker-host").
		SetType(agenthost.TypeDocker).
		SetStatus(agenthost.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	localHost, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, "Local", localHost.Name)
	assert.Equal(t, filepath.Join(homeDir, ".axonclaw", "hosts"), localHost.Directory)

	totalCount, err := client.AgentHost.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, totalCount)

	localCount, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, localCount)
}
