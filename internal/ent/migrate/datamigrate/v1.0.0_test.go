package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/agentruntime"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
)

func TestV1_0_0_CreateLocalAgentRuntime(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	err := datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	runtime, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Local", runtime.Name)
	assert.Equal(t, agentruntime.TypeLocal, runtime.Type)
	assert.Equal(t, agentruntime.StatusActive, runtime.Status)
}

func TestV1_0_0_LocalAgentRuntimeAlreadyExists(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	existingRuntime, err := client.AgentRuntime.Create().
		SetName("existing-local").
		SetType(agentruntime.TypeLocal).
		SetStatus(agentruntime.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	count, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	runtime, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, existingRuntime.ID, runtime.ID)
	assert.Equal(t, "existing-local", runtime.Name)
}

func TestV1_0_0_Idempotency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	err := datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	runtime1, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	count, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	runtime2, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, runtime1.ID, runtime2.ID)
}

func TestV1_0_0_VerifyAgentRuntimeFields(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	err := datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	runtime, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)

	assert.NotZero(t, runtime.ID)
	assert.NotZero(t, runtime.CreatedAt)
	assert.NotZero(t, runtime.UpdatedAt)
	assert.Equal(t, "Local", runtime.Name)
	assert.Equal(t, agentruntime.TypeLocal, runtime.Type)
	assert.Equal(t, agentruntime.StatusActive, runtime.Status)
}

func TestV1_0_0_MultipleNonLocalAgentRuntimes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	_, err := client.AgentRuntime.Create().
		SetName("vm-runtime").
		SetType(agentruntime.TypeVM).
		SetStatus(agentruntime.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.AgentRuntime.Create().
		SetName("docker-runtime").
		SetType(agentruntime.TypeDocker).
		SetStatus(agentruntime.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV1_0_0().Migrate(ctx, client)
	require.NoError(t, err)

	localRuntime, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Local", localRuntime.Name)

	totalCount, err := client.AgentRuntime.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, totalCount)

	localCount, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, localCount)
}
