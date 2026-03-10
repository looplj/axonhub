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
	"github.com/looplj/axonhub/internal/ent/agenthost"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

func makeTestBaseEnv(t *testing.T) (context.Context, *ent.Client, *ent.User, *ent.Project, *ent.Agent) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

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

	return ctx, client, u, p, a
}

// TestAgentDeployService_DeployAxonClawByAgent_RejectsLocalHostWithoutDirectory checks that
// deploying to a local host that has no directory configured fails validation.
func TestAgentDeployService_DeployAxonClawByAgent_RejectsLocalHostWithoutDirectory(t *testing.T) {
	ctx, client, u, p, a := makeTestBaseEnv(t)

	h, err := client.AgentHost.Create().
		SetName("Local").
		SetType(agenthost.TypeLocal).
		SetStatus(agenthost.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	k, err := GenerateAPIKey()
	require.NoError(t, err)

	currentAPIKey, err := client.APIKey.Create().
		SetUserID(u.ID).
		SetProjectID(p.ID).
		SetKey(k).
		SetName("current agent api key").
		SetType(apikey.TypeAgent).
		Save(ctx)
	require.NoError(t, err)

	currentInst, err := client.AgentInstance.Create().
		SetProjectID(p.ID).
		SetAgentID(a.ID).
		SetHostID(h.ID).
		SetAPIKeyID(currentAPIKey.ID).
		SetName("current").
		SetAxonhubBaseURL("http://localhost:8090").
		SetStatus(agentinstance.StatusRunning).
		SetLastHeartbeatAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	svc := NewAgentDeployService(AgentDeployServiceParams{Ent: client})

	result, err := svc.DeployAxonClawByAgent(ctx, currentInst, DeployAxonClawByAgentInput{
		Name: "worker-1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Success)
	require.Contains(t, result.Error, "host directory is required")
}

// TestAgentDeployService_DeployAxonClawByAgent_RejectsDuplicateInstanceName checks that
// deploying with a name that already exists for the same agent fails.
func TestAgentDeployService_DeployAxonClawByAgent_RejectsDuplicateInstanceName(t *testing.T) {
	ctx, client, u, p, a := makeTestBaseEnv(t)

	h, err := client.AgentHost.Create().
		SetName("Docker").
		SetType(agenthost.TypeDocker).
		SetStatus(agenthost.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	k, err := GenerateAPIKey()
	require.NoError(t, err)

	currentAPIKey, err := client.APIKey.Create().
		SetUserID(u.ID).
		SetProjectID(p.ID).
		SetKey(k).
		SetName("current agent api key").
		SetType(apikey.TypeAgent).
		Save(ctx)
	require.NoError(t, err)

	currentName := "worker-1"

	currentInst, err := client.AgentInstance.Create().
		SetProjectID(p.ID).
		SetAgentID(a.ID).
		SetHostID(h.ID).
		SetAPIKeyID(currentAPIKey.ID).
		SetName(currentName).
		SetAxonhubBaseURL("http://localhost:8090").
		SetStatus(agentinstance.StatusRunning).
		SetLastHeartbeatAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	svc := NewAgentDeployService(AgentDeployServiceParams{Ent: client})

	result, err := svc.DeployAxonClawByAgent(ctx, currentInst, DeployAxonClawByAgentInput{
		Name: currentName,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Success)
}
