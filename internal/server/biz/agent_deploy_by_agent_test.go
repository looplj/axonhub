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

func TestAgentDeployService_DeployAxonClawByAgent_RejectsSameDirectory(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

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

	currentDir := "/tmp/axonclaw/current"

	currentInst, err := client.AgentInstance.Create().
		SetProjectID(p.ID).
		SetAgentID(a.ID).
		SetHostID(h.ID).
		SetAPIKeyID(currentAPIKey.ID).
		SetName("current").
		SetStatus(agentinstance.StatusRunning).
		SetDeployment(objects.AgentInstanceDeployment{
			Directory:      currentDir,
			AxonhubBaseURL: "http://localhost:8090",
		}).
		SetLastHeartbeatAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	svc := NewAgentDeployService(AgentDeployServiceParams{Ent: client})

	result, err := svc.DeployAxonClawByAgent(ctx, currentInst, DeployAxonClawByAgentInput{
		Name:      "worker-1",
		Directory: &currentDir,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Success)
	require.Contains(t, result.Error, "directory must be different")
}

func TestAgentDeployService_DeployAxonClawByAgent_RejectsSameDockerContainer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

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
		SetStatus(agentinstance.StatusRunning).
		SetDeployment(objects.AgentInstanceDeployment{
			DockerContainerName: dockerContainerName(currentName),
			AxonhubBaseURL:      "http://localhost:8090",
		}).
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
	require.Contains(t, result.Error, "target docker container")
}
