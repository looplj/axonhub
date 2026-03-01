package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

func TestAgentBootstrapService_HeartbeatAgentInstance_UpdatesStatus(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	u, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	p, err := client.Project.Create().
		SetName(uuid.NewString()).
		SetDescription("test project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	sysPrompt, err := client.Prompt.Create().
		SetProjectID(p.ID).
		SetType(prompt.TypeAgent).
		SetName("agent prompt").
		SetRole("system").
		SetContent("test").
		SetStatus(prompt.StatusEnabled).
		SetSettings(objects.PromptSettings{Action: objects.PromptAction{Type: objects.PromptActionTypeNoop}}).
		Save(ctx)
	require.NoError(t, err)

	a, err := client.Agent.Create().
		SetProjectID(p.ID).
		SetCreatedByUserID(u.ID).
		SetPromptID(sysPrompt.ID).
		SetName("agent").
		SetStatus(agent.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	k, err := GenerateAPIKey()
	require.NoError(t, err)

	apiKey, err := client.APIKey.Create().
		SetUserID(u.ID).
		SetProjectID(p.ID).
		SetKey(k).
		SetName("agent api key").
		SetType(apikey.TypeAgent).
		Save(ctx)
	require.NoError(t, err)

	old := time.Now().Add(-time.Minute)

	inst, err := client.AgentInstance.Create().
		SetProjectID(p.ID).
		SetAgentID(a.ID).
		SetAPIKeyID(apiKey.ID).
		SetName("inst").
		SetStatus(agentinstance.StatusStopped).
		SetLastHeartbeatAt(old).
		Save(ctx)
	require.NoError(t, err)

	svc := NewAgentBootstrapService(AgentBootstrapServiceParams{Ent: client})

	err = svc.HeartbeatAgentInstance(ctx, inst)
	require.NoError(t, err)

	got, err := client.AgentInstance.Get(ctx, inst.ID)
	require.NoError(t, err)
	require.Equal(t, agentinstance.StatusRunning, got.Status)
	require.True(t, got.LastHeartbeatAt.After(old))
	require.WithinDuration(t, time.Now(), got.LastHeartbeatAt, 5*time.Second)
}
