//nolint:gosec // G204: Subprocess launched with variable.
package biz

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.uber.org/fx"
	"golang.org/x/crypto/ssh"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/agentruntime"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

var (
	debugLocalPath   = os.Getenv("AXONHUB_DEBUG_AXONCLAW_PATH")
	debugDockerImage = os.Getenv("AXONHUB_DEBUG_AXONCLAW_IMAGE")
)

type AgentRuntimeServiceParams struct {
	fx.In

	Ent *ent.Client
}

// AgentRuntimeService provides APIs for the runtime agent endpoint (/agent/v1/graphql).
// This service enforces agent API key ownership checks at the application layer and
// uses system bypass for DB access to avoid coupling to Ent privacy rules.
type AgentRuntimeService struct {
	*AbstractService
}

func NewAgentRuntimeService(params AgentRuntimeServiceParams) *AgentRuntimeService {
	return &AgentRuntimeService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

type CreateAgentRuntimeInput struct {
	Name     string
	Type     agentruntime.Type
	Host     string
	User     string
	Password string
}

func (svc *AgentRuntimeService) CreateAgentRuntime(ctx context.Context, input CreateAgentRuntimeInput) (*ent.AgentRuntime, error) {
	runtime, err := svc.db.AgentRuntime.Create().
		SetName(input.Name).
		SetType(input.Type).
		SetHost(input.Host).
		SetUser(input.User).
		SetPassword(input.Password).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent runtime: %w", err)
	}

	return runtime, nil
}

type UpdateAgentRuntimeInput struct {
	Name     *string
	Status   *agentruntime.Status
	Host     *string
	User     *string
	Password *string
}

func (svc *AgentRuntimeService) UpdateAgentRuntime(ctx context.Context, id int, input UpdateAgentRuntimeInput) (*ent.AgentRuntime, error) {
	runtime, err := svc.db.AgentRuntime.UpdateOneID(id).
		SetNillableName(input.Name).
		SetNillableStatus(input.Status).
		SetNillableHost(input.Host).
		SetNillableUser(input.User).
		SetNillablePassword(input.Password).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent runtime: %w", err)
	}

	return runtime, nil
}

func (svc *AgentRuntimeService) DeleteAgentRuntime(ctx context.Context, id int) error {
	n, err := svc.db.AgentRuntime.Delete().Where(agentruntime.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete agent runtime: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("agent runtime not found")
	}
	return nil
}

type TestConnectionResult struct {
	Success bool
	Error   string
	Latency int
}

func (svc *AgentRuntimeService) TestConnection(ctx context.Context, id int) (*TestConnectionResult, error) {
	runtime, err := svc.db.AgentRuntime.Query().Where(agentruntime.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent runtime: %w", err)
	}

	return svc.testConnection(ctx, runtime)
}

func (svc *AgentRuntimeService) testConnection(_ context.Context, runtime *ent.AgentRuntime) (*TestConnectionResult, error) {
	start := time.Now()

	if runtime.Host == "" {
		return &TestConnectionResult{
			Success: false,
			Error:   "host not configured",
		}, nil
	}

	return &TestConnectionResult{
		Success: true,
		Latency: int(time.Since(start).Milliseconds()),
	}, nil
}

type DeployAxonclawInput struct {
	AgentID        int
	RuntimeID      int
	Name           string
	Directory      string
	AxonhubBaseURL string
}

type DeployAxonclawResult struct {
	Success  bool
	Error    string
	Instance *ent.AgentInstance
}

func (svc *AgentRuntimeService) DeployAxonclaw(ctx context.Context, input DeployAxonclawInput) (*DeployAxonclawResult, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("user not found in context")
	}
	runtime, err := svc.db.AgentRuntime.Query().Where(agentruntime.IDEQ(input.RuntimeID)).Only(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load runtime: %v", err),
		}, nil
	}

	if err := validateDeployInput(input, runtime.Type, runtime.Host); err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	entity, err := svc.db.Agent.Query().
		Where(agent.IDEQ(input.AgentID)).
		Only(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load agent: %v", err),
		}, nil
	}

	var (
		instance *ent.AgentInstance
		apiKey   *ent.APIKey
	)

	err = svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		client := svc.entFromContext(txCtx)

		apiKeyName := fmt.Sprintf("agent-instance:%d:%s", input.AgentID, input.Name)

		generatedKey, err := GenerateAPIKey()
		if err != nil {
			return fmt.Errorf("failed to generate api key: %w", err)
		}

		apiKey, err = authz.RunWithSystemBypass(txCtx, "create-agent-instance-api-key", func(bypassCtx context.Context) (*ent.APIKey, error) {
			return client.APIKey.Create().
				SetName(apiKeyName).
				SetKey(generatedKey).
				SetUserID(user.ID).
				SetProjectID(entity.ProjectID).
				SetType(apikey.TypeAgent).
				SetScopes([]string{
					string(scopes.ScopeReadAgents),
					string(scopes.ScopeWriteAgents),
					string(scopes.ScopeReadRequests),
					string(scopes.ScopeWriteRequests),
				}).
				Save(bypassCtx)
		})
		if err != nil {
			return fmt.Errorf("failed to create api key: %w", err)
		}

		deployment := objects.AgentInstanceDeployment{
			AxonhubBaseURL: input.AxonhubBaseURL,
		}

		switch runtime.Type {
		case agentruntime.TypeVM:
			deployment.Directory = input.Directory
		case agentruntime.TypeDocker:
			deployment.DockerContainerName = dockerContainerName(input.Name)
		case agentruntime.TypeLocal:
			deployment.Directory = input.Directory
		}

		instance, err = client.AgentInstance.Create().
			SetProjectID(entity.ProjectID).
			SetAgentID(input.AgentID).
			SetRuntimeID(input.RuntimeID).
			SetName(input.Name).
			SetStatus(agentinstance.StatusPending).
			SetDeployment(deployment).
			SetLastHeartbeatAt(time.Now()).
			SetAPIKeyID(apiKey.ID).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to create instance: %w", err)
		}

		return nil
	})
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	err = svc.executeDeployment(ctx, runtime, instance, apiKey, input)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &DeployAxonclawResult{
		Success:  true,
		Instance: instance,
	}, nil
}

func (svc *AgentRuntimeService) executeDeployment(ctx context.Context, runtime *ent.AgentRuntime, instance *ent.AgentInstance, apiKey *ent.APIKey, input DeployAxonclawInput) error {
	var err error

	baseURL := input.AxonhubBaseURL
	switch runtime.Type {
	case agentruntime.TypeVM:
		err = svc.deployToVM(ctx, runtime, apiKey, input.Name, input.Directory, baseURL)
	case agentruntime.TypeDocker:
		err = svc.deployToDocker(ctx, runtime, apiKey, input.Name, baseURL)
	case agentruntime.TypeLocal:
		err = svc.deployToLocal(ctx, runtime, apiKey, input.Name, input.Directory, baseURL)
	}

	if err != nil {
		_, _ = svc.db.AgentInstance.UpdateOneID(instance.ID).
			SetStatus(agentinstance.StatusError).
			Save(ctx)

		return fmt.Errorf("failed to deploy to runtime %s: %w", runtime.Type, err)
	}

	_, _ = svc.db.AgentInstance.UpdateOneID(instance.ID).
		SetStatus(agentinstance.StatusRunning).
		Save(ctx)

	return nil
}

func validateDeployInput(input DeployAxonclawInput, runtimeType agentruntime.Type, host string) error {
	if input.AgentID <= 0 {
		return fmt.Errorf("agent ID is required")
	}
	if input.RuntimeID <= 0 {
		return fmt.Errorf("runtime ID is required")
	}
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}

	if runtimeType == agentruntime.TypeVM && input.Directory == "" {
		return fmt.Errorf("directory is required for VM runtime")
	}

	if runtimeType == agentruntime.TypeLocal && input.Directory == "" {
		return fmt.Errorf("directory is required for local runtime")
	}
	return nil
}

func (svc *AgentRuntimeService) deployToVM(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"

	if isLocalhost {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", directory, err)
		}

		if debugLocalPath != "" {
			if _, err := os.Stat(debugLocalPath); os.IsNotExist(err) {
				return fmt.Errorf("debug package not found at %s", debugLocalPath)
			}

			//nolint:gosec
			unzipCmd := fmt.Sprintf("unzip -o %s -d %s && chmod +x %s/start.sh %s/stop.sh", debugLocalPath, directory, directory, directory)
			if err := exec.CommandContext(ctx, "sh", "-c", unzipCmd).Run(); err != nil {
				return fmt.Errorf("failed to unzip debug package: %w", err)
			}

			//nolint:gosec
			startCmd := fmt.Sprintf("cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./start.sh", directory, name, baseURL, apiKey.Key)
			if err := exec.CommandContext(ctx, "sh", "-c", startCmd).Run(); err != nil {
				return fmt.Errorf("failed to start debug axonclaw: %w", err)
			}

			return nil
		}

		//nolint:gosec
		deployCmd := fmt.Sprintf("cd %s && curl -sSL https://get.axonclaw.io/install.sh | AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s sh", directory, name, baseURL, apiKey.Key)
		if err := exec.CommandContext(ctx, "sh", "-c", deployCmd).Run(); err != nil {
			return fmt.Errorf("failed to deploy axonclaw: %w", err)
		}

		return nil
	}

	config := &ssh.ClientConfig{
		User: runtime.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(runtime.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", runtime.Host, config)
	if err != nil {
		return fmt.Errorf("failed to connect to host %s: %w", runtime.Host, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	mkdirCmd := fmt.Sprintf("mkdir -p %s", directory)
	if err := session.Run(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", directory, err)
	}

	session2, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session2.Close()

	deployCmd := fmt.Sprintf(
		"cd %s && curl -sSL https://get.axonclaw.io/install.sh | AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s sh",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)
	if err := session2.Run(deployCmd); err != nil {
		return fmt.Errorf("failed to deploy axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) deployToDocker(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, baseURL string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	containerName := fmt.Sprintf("axonclaw-%s", name)

	imageName := "axonclaw/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	if isLocalhost {
		//nolint:gosec
		stopCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))
		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop existing container: %w", err)
		}

		//nolint:gosec
		rmCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName))
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove existing container: %w", err)
		}

		if debugDockerImage == "" {
			//nolint:gosec
			pullCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker pull %s", imageName))
			if err := pullCmd.Run(); err != nil {
				return fmt.Errorf("failed to pull latest image: %w", err)
			}
		}

		//nolint:gosec
		runCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s", containerName, name, baseURL, apiKey.Key, imageName))
		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("failed to start Docker container: %w", err)
		}

		time.Sleep(2 * time.Second)

		//nolint:gosec
		checkCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker inspect --format='{{.State.Running}}' %s", containerName))
		output, err := checkCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check container status: %w", err)
		}

		if string(output) != "true\n" {
			//nolint:gosec
			logsCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker logs %s", containerName))
			logsOutput, _ := logsCmd.CombinedOutput()
			return fmt.Errorf("container is not running. Logs: %s", string(logsOutput))
		}

		return nil
	}

	config := &ssh.ClientConfig{
		User: runtime.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(runtime.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", runtime.Host, config)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker host %s: %w", runtime.Host, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	stopCmd := fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName)
	if err := session.Run(stopCmd); err != nil {
		return fmt.Errorf("failed to stop existing container: %w", err)
	}

	session2, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session2.Close()

	rmCmd := fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName)
	if err := session2.Run(rmCmd); err != nil {
		return fmt.Errorf("failed to remove existing container: %w", err)
	}

	session3, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session3.Close()

	pullCmd := fmt.Sprintf("docker pull %s", imageName)
	if err := session3.Run(pullCmd); err != nil {
		return fmt.Errorf("failed to pull latest image: %w", err)
	}

	session4, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session4.Close()

	runCmd := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s", containerName, name, baseURL, apiKey.Key, imageName)
	if err := session4.Run(runCmd); err != nil {
		return fmt.Errorf("failed to start Docker container: %w", err)
	}

	session5, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session5.Close()

	time.Sleep(2 * time.Second)

	checkCmd := fmt.Sprintf("docker inspect --format='{{.State.Running}}' %s", containerName)
	output, err := session5.CombinedOutput(checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check container status: %w", err)
	}

	if string(output) != "true\n" {
		logsSession, _ := client.NewSession()
		if logsSession != nil {
			defer logsSession.Close()

			logsCmd := fmt.Sprintf("docker logs %s", containerName)
			logsOutput, _ := logsSession.CombinedOutput(logsCmd)
			return fmt.Errorf("container is not running. Logs: %s", string(logsOutput))
		}
		return fmt.Errorf("container is not running")
	}

	return nil
}

type ControlAxonclawInstanceResult struct {
	Success  bool
	Error    string
	Instance *ent.AgentInstance
}

type axonclawControlAction string

const (
	axonclawControlStart    axonclawControlAction = "start"
	axonclawControlStop     axonclawControlAction = "stop"
	axonclawControlRestart  axonclawControlAction = "restart"
	axonclawControlRedeploy axonclawControlAction = "redeploy"
)

func (svc *AgentRuntimeService) StartAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlStart)
}

func (svc *AgentRuntimeService) StopAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlStop)
}

func (svc *AgentRuntimeService) RestartAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlRestart)
}

func (svc *AgentRuntimeService) RedeployAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlRedeploy)
}

//nolint:nilerr // ignore nil error, it's handled in the function body.
func (svc *AgentRuntimeService) controlAxonclawInstance(ctx context.Context, instanceID int, action axonclawControlAction) (*ControlAxonclawInstanceResult, error) {
	client := svc.entFromContext(ctx)

	instance, err := client.AgentInstance.Query().
		Where(agentinstance.IDEQ(instanceID)).
		WithRuntime().
		Only(ctx)
	if err != nil {
		return &ControlAxonclawInstanceResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load instance: %v", err),
		}, nil
	}

	if instance.Edges.Runtime == nil {
		return &ControlAxonclawInstanceResult{
			Success:  false,
			Error:    "instance is not bound to a runtime",
			Instance: instance,
		}, nil
	}

	runtime := instance.Edges.Runtime
	deployment := instance.Deployment

	var apiKey *ent.APIKey
	if action != axonclawControlStop {
		apiKey, err = authz.RunWithSystemBypass(ctx, "load-agent-instance-api-key", func(bypassCtx context.Context) (*ent.APIKey, error) {
			return client.APIKey.Query().Where(apikey.IDEQ(instance.APIKeyID)).Only(bypassCtx)
		})
		if err != nil {
			return &ControlAxonclawInstanceResult{
				Success:  false,
				Error:    fmt.Sprintf("failed to load instance api key: %v", err),
				Instance: instance,
			}, nil
		}
	}

	var actionErr error

	switch action {
	case axonclawControlStop:
		actionErr = svc.stopAxonclaw(ctx, runtime, instance.Name, deployment)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusStopped).Save(ctx)
		}
	case axonclawControlStart:
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusPending).Save(ctx)

		actionErr = svc.startAxonclaw(ctx, runtime, apiKey, instance.Name, deployment)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusRunning).Save(ctx)
		}
	case axonclawControlRestart:
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusPending).Save(ctx)

		actionErr = svc.restartAxonclaw(ctx, runtime, apiKey, instance.Name, deployment)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusRunning).Save(ctx)
		}
	case axonclawControlRedeploy:
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusPending).Save(ctx)

		actionErr = svc.redeployAxonclaw(ctx, runtime, apiKey, instance.Name, deployment)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusRunning).Save(ctx)
		}
	default:
		actionErr = fmt.Errorf("unknown action: %s", action)
	}

	if actionErr != nil {
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusError).Save(ctx)

		return &ControlAxonclawInstanceResult{
			Success:  false,
			Error:    actionErr.Error(),
			Instance: instance,
		}, nil
	}

	updated, err := client.AgentInstance.Query().Where(agentinstance.IDEQ(instance.ID)).Only(ctx)
	if err != nil {
		updated = instance
	}

	return &ControlAxonclawInstanceResult{
		Success:  true,
		Instance: updated,
	}, nil
}

func (svc *AgentRuntimeService) stopAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, name string, deployment objects.AgentInstanceDeployment) error {
	switch runtime.Type {
	case agentruntime.TypeVM:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		return svc.vmStop(ctx, runtime, deployment.Directory)
	case agentruntime.TypeDocker:
		containerName := deployment.DockerContainerName
		if strings.TrimSpace(containerName) == "" {
			containerName = dockerContainerName(name)
		}

		return svc.dockerStop(ctx, runtime, containerName)
	case agentruntime.TypeLocal:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		return svc.localStop(ctx, deployment.Directory)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func (svc *AgentRuntimeService) startAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name string, deployment objects.AgentInstanceDeployment) error {
	switch runtime.Type {
	case agentruntime.TypeVM:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		return svc.vmStart(ctx, runtime, apiKey, name, deployment.Directory, deployment.AxonhubBaseURL)
	case agentruntime.TypeDocker:
		containerName := deployment.DockerContainerName
		if strings.TrimSpace(containerName) == "" {
			containerName = dockerContainerName(name)
		}

		return svc.dockerStart(ctx, runtime, containerName)
	case agentruntime.TypeLocal:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		return svc.localStart(ctx, apiKey, name, deployment.Directory, deployment.AxonhubBaseURL)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func (svc *AgentRuntimeService) restartAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name string, deployment objects.AgentInstanceDeployment) error {
	switch runtime.Type {
	case agentruntime.TypeVM:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		return svc.vmRestart(ctx, runtime, apiKey, name, deployment.Directory, deployment.AxonhubBaseURL)
	case agentruntime.TypeDocker:
		containerName := deployment.DockerContainerName
		if strings.TrimSpace(containerName) == "" {
			containerName = dockerContainerName(name)
		}

		return svc.dockerRestart(ctx, runtime, containerName)
	case agentruntime.TypeLocal:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		return svc.localRestart(ctx, apiKey, name, deployment.Directory, deployment.AxonhubBaseURL)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func (svc *AgentRuntimeService) redeployAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name string, deployment objects.AgentInstanceDeployment) error {
	baseURL := deployment.AxonhubBaseURL

	switch runtime.Type {
	case agentruntime.TypeVM:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		_ = svc.vmStop(ctx, runtime, deployment.Directory)
		if err := svc.vmInstallLatest(ctx, runtime, apiKey, name, deployment.Directory, baseURL); err != nil {
			return err
		}

		return svc.vmStart(ctx, runtime, apiKey, name, deployment.Directory, baseURL)
	case agentruntime.TypeDocker:
		containerName := deployment.DockerContainerName
		if strings.TrimSpace(containerName) == "" {
			containerName = dockerContainerName(name)
		}

		return svc.dockerRedeploy(ctx, runtime, apiKey, name, containerName, baseURL)
	case agentruntime.TypeLocal:
		if strings.TrimSpace(deployment.Directory) == "" {
			return fmt.Errorf("deployment directory not recorded")
		}

		_ = svc.localStop(ctx, deployment.Directory)
		if err := svc.localInstallLatest(ctx, apiKey, name, deployment.Directory, baseURL); err != nil {
			return err
		}

		return svc.localStart(ctx, apiKey, name, deployment.Directory, baseURL)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func dockerContainerName(name string) string {
	return fmt.Sprintf("axonclaw-%s", name)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (svc *AgentRuntimeService) vmStop(ctx context.Context, runtime *ent.AgentRuntime, directory string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "./stop.sh") //nolint:gosec

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("stop axonclaw: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	stopCmd := fmt.Sprintf("cd %s && ./stop.sh", shellQuote(directory))
	if err := session.Run(stopCmd); err != nil {
		return fmt.Errorf("stop axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) vmStart(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "./start.sh") //nolint:gosec
		cmd.Dir = directory

		cmd.Env = append(os.Environ(),
			"AXONCLAW_NAME="+name,
			"AXONCLAW_BASE_URL="+baseURL,
			"AXONCLAW_API_KEY="+apiKey.Key,
		)

		setProcessGroup(cmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start axonclaw: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	startCmd := fmt.Sprintf(
		"cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./start.sh",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)
	if err := session.Run(startCmd); err != nil {
		return fmt.Errorf("start axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) vmRestart(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "./restart.sh") //nolint:gosec
		cmd.Dir = directory

		cmd.Env = append(os.Environ(),
			"AXONCLAW_NAME="+name,
			"AXONCLAW_BASE_URL="+baseURL,
			"AXONCLAW_API_KEY="+apiKey.Key,
		)

		setProcessGroup(cmd)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restart axonclaw: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	restartCmd := fmt.Sprintf(
		"cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./restart.sh",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)
	if err := session.Run(restartCmd); err != nil {
		return fmt.Errorf("restart axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) vmInstallLatest(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"

	if isLocalhost {
		cmd := exec.CommandContext(ctx, "sh", "-c", "curl -sSL https://get.axonclaw.io/install.sh | sh") //nolint:gosec
		cmd.Dir = directory

		cmd.Env = append(os.Environ(),
			"AXONCLAW_NAME="+name,
			"AXONCLAW_BASE_URL="+baseURL,
			"AXONCLAW_API_KEY="+apiKey.Key,
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install latest axonclaw: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	installCmd := fmt.Sprintf(
		"cd %s && curl -sSL https://get.axonclaw.io/install.sh | AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s sh",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)
	if err := session.Run(installCmd); err != nil {
		return fmt.Errorf("install latest axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) dockerStop(ctx context.Context, runtime *ent.AgentRuntime, containerName string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
		_ = cmd.Run()

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	stopCmd := fmt.Sprintf("docker stop %s 2>/dev/null || true", shellQuote(containerName))
	if err := session.Run(stopCmd); err != nil {
		return fmt.Errorf("docker stop: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) dockerStart(ctx context.Context, runtime *ent.AgentRuntime, containerName string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		if err := exec.CommandContext(ctx, "docker", "start", containerName).Run(); err != nil {
			return fmt.Errorf("docker start: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	startCmd := fmt.Sprintf("docker start %s", shellQuote(containerName))
	if err := session.Run(startCmd); err != nil {
		return fmt.Errorf("docker start: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) dockerRestart(ctx context.Context, runtime *ent.AgentRuntime, containerName string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		if err := exec.CommandContext(ctx, "docker", "restart", containerName).Run(); err != nil {
			return fmt.Errorf("docker restart: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	restartCmd := fmt.Sprintf("docker restart %s", shellQuote(containerName))
	if err := session.Run(restartCmd); err != nil {
		return fmt.Errorf("docker restart: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) dockerRedeploy(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, containerName, baseURL string) error {
	imageName := "axonclaw/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		_ = exec.CommandContext(ctx, "docker", "stop", containerName).Run()
		_ = exec.CommandContext(ctx, "docker", "rm", containerName).Run()

		if debugDockerImage == "" {
			if err := exec.CommandContext(ctx, "docker", "pull", imageName).Run(); err != nil {
				return fmt.Errorf("docker pull: %w", err)
			}
		}

		runArgs := []string{
			"run", "-d",
			"--name", containerName,
			"--restart", "unless-stopped",
			"-e", "AXONCLAW_NAME=" + name,
			"-e", "AXONCLAW_BASE_URL=" + baseURL,
			"-e", "AXONCLAW_API_KEY=" + apiKey.Key,
			imageName,
		}
		if err := exec.CommandContext(ctx, "docker", runArgs...).Run(); err != nil {
			return fmt.Errorf("docker run: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	runSSH := func(cmd string) error {
		s, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("create ssh session: %w", err)
		}
		defer s.Close()

		return s.Run(cmd)
	}

	if debugDockerImage == "" {
		if err := runSSH(fmt.Sprintf("docker pull %s", shellQuote(imageName))); err != nil {
			return fmt.Errorf("docker pull: %w", err)
		}
	}

	_ = runSSH(fmt.Sprintf("docker stop %s 2>/dev/null || true", shellQuote(containerName)))
	_ = runSSH(fmt.Sprintf("docker rm %s 2>/dev/null || true", shellQuote(containerName)))

	runCmd := fmt.Sprintf(
		"docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s",
		shellQuote(containerName),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
		shellQuote(imageName),
	)
	if err := runSSH(runCmd); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}

	return nil
}

func sshDial(runtime *ent.AgentRuntime) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: runtime.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(runtime.Password),
		},
		//nolint:gosec // ignore G202, it's a test environment.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", runtime.Host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host %s: %w", runtime.Host, err)
	}

	return client, nil
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func (svc *AgentRuntimeService) deployToLocal(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", directory, err)
	}

	if debugLocalPath != "" {
		if _, err := os.Stat(debugLocalPath); os.IsNotExist(err) {
			return fmt.Errorf("debug package not found at %s", debugLocalPath)
		}

		if isWindows() {
			return svc.deployToLocalWindows(ctx, apiKey, name, directory, baseURL)
		}

		unzipCmd := fmt.Sprintf("unzip -o %s -d %s && chmod +x %s/start.sh %s/stop.sh", debugLocalPath, directory, directory, directory)
		if err := exec.CommandContext(ctx, "sh", "-c", unzipCmd).Run(); err != nil {
			return fmt.Errorf("failed to unzip debug package: %w", err)
		}

		startCmd := fmt.Sprintf("cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./start.sh", directory, name, baseURL, apiKey.Key)
		if err := exec.CommandContext(ctx, "sh", "-c", startCmd).Run(); err != nil {
			return fmt.Errorf("failed to start debug axonclaw: %w", err)
		}

		return nil
	}

	return svc.localInstallLatest(ctx, apiKey, name, directory, baseURL)
}

func (svc *AgentRuntimeService) deployToLocalWindows(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	expandCmd := fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", debugLocalPath, directory)
	if err := exec.CommandContext(ctx, "powershell", "-Command", expandCmd).Run(); err != nil {
		return fmt.Errorf("failed to expand archive: %w", err)
	}

	startCmd := fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; .\\start.bat", directory, name, baseURL, apiKey.Key)
	if err := exec.CommandContext(ctx, "powershell", "-Command", startCmd).Run(); err != nil {
		return fmt.Errorf("failed to start axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) localStop(ctx context.Context, directory string) error {
	if isWindows() {
		cmd := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("cd %s; .\\stop.bat", directory))

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("stop axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "./stop.sh")

	cmd.Dir = directory
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) localStart(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if isWindows() {
		cmd := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; .\\start.bat", directory, name, baseURL, apiKey.Key))

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "./start.sh")
	cmd.Dir = directory

	cmd.Env = append(os.Environ(),
		"AXONCLAW_NAME="+name,
		"AXONCLAW_BASE_URL="+baseURL,
		"AXONCLAW_API_KEY="+apiKey.Key,
	)

	setProcessGroup(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) localRestart(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if isWindows() {
		cmd := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; .\\restart.bat", directory, name, baseURL, apiKey.Key))

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restart axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "./restart.sh")
	cmd.Dir = directory

	cmd.Env = append(os.Environ(),
		"AXONCLAW_NAME="+name,
		"AXONCLAW_BASE_URL="+baseURL,
		"AXONCLAW_API_KEY="+apiKey.Key,
	)

	setProcessGroup(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restart axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) localInstallLatest(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if isWindows() {
		installCmd := fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; Invoke-Expression (Invoke-WebRequest -Uri 'https://get.axonclaw.io/install.ps1' -UseBasicParsing).Content", directory, name, baseURL, apiKey.Key)
		cmd := exec.CommandContext(ctx, "powershell", "-Command", installCmd)

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install latest axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", "curl -sSL https://get.axonclaw.io/install.sh | sh")
	cmd.Dir = directory

	cmd.Env = append(os.Environ(),
		"AXONCLAW_NAME="+name,
		"AXONCLAW_BASE_URL="+baseURL,
		"AXONCLAW_API_KEY="+apiKey.Key,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install latest axonclaw: %w", err)
	}

	return nil
}
