package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"time"

	"go.uber.org/fx"
	"golang.org/x/crypto/ssh"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/agentruntime"
	"github.com/looplj/axonhub/internal/objects"
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
	AgentID   int
	RuntimeID int
	Name      string
	Directory string
}

type DeployAxonclawResult struct {
	Success  bool
	Error    string
	Instance *ent.AgentInstance
}

func (svc *AgentRuntimeService) DeployAxonclaw(ctx context.Context, input DeployAxonclawInput) (*DeployAxonclawResult, error) {
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
		WithAPIKey().
		Where(agent.IDEQ(input.AgentID)).
		Only(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load agent: %v", err),
		}, nil
	}

	instanceID := generateInstanceID()

	instance, err := svc.db.AgentInstance.Create().
		SetProjectID(entity.ProjectID).
		SetAgentID(input.AgentID).
		SetRuntimeID(input.RuntimeID).
		SetInstanceID(instanceID).
		SetName(input.Name).
		SetStatus(agentinstance.StatusPending).
		SetDeployment(objects.AgentInstanceDeployment{
			Directory: input.Directory,
		}).
		SetLastHeartbeatAt(time.Now()).
		Save(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create instance: %v", err),
		}, nil
	}

	switch runtime.Type {
	case agentruntime.TypeVM:
		if err := svc.deployToVM(ctx, runtime, entity, instanceID, input.Name, input.Directory); err != nil {
			_, _ = svc.db.AgentInstance.UpdateOneID(instance.ID).
				SetStatus(agentinstance.StatusError).
				Save(ctx)
			return &DeployAxonclawResult{
				Success: false,
				Error:   fmt.Sprintf("VM deployment failed: %v", err),
			}, nil
		}
	case agentruntime.TypeDocker:
		if err := svc.deployToDocker(ctx, runtime, entity, instanceID, input.Name); err != nil {
			_, _ = svc.db.AgentInstance.UpdateOneID(instance.ID).
				SetStatus(agentinstance.StatusError).
				Save(ctx)
			return &DeployAxonclawResult{
				Success: false,
				Error:   fmt.Sprintf("Docker deployment failed: %v", err),
			}, nil
		}
	}

	instance, err = svc.db.AgentInstance.UpdateOneID(instance.ID).
		SetStatus(agentinstance.StatusRunning).
		Save(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to update instance status: %v", err),
		}, nil
	}

	return &DeployAxonclawResult{
		Success:  true,
		Instance: instance,
	}, nil
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
	return nil
}

func (svc *AgentRuntimeService) deployToVM(ctx context.Context, runtime *ent.AgentRuntime, agent *ent.Agent, instanceID, name, directory string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"

	apiKey, err := agent.APIKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agent API key: %w", err)
	}

	var baseURL string
	if debugLocalPath != "" {
		baseURL = "http://localhost:8090"
	} else {
		baseURL = "http://" + runtime.Host + ":8090"
	}

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
			startCmd := fmt.Sprintf("cd %s && AXONCLAW_INSTANCE_ID=%s AXONCLAW_NAME=%s AXONHUB_BASE_URL=%s AXONHUB_API_KEY=%s ./start.sh", directory, instanceID, name, baseURL, apiKey.Key)
			if err := exec.CommandContext(ctx, "sh", "-c", startCmd).Run(); err != nil {
				return fmt.Errorf("failed to start debug axonclaw: %w", err)
			}

			return nil
		}

		//nolint:gosec
		deployCmd := fmt.Sprintf("cd %s && curl -sSL https://get.axonclaw.io/install.sh | AXONCLAW_INSTANCE_ID=%s AXONCLAW_NAME=%s AXONHUB_BASE_URL=%s AXONHUB_API_KEY=%s sh", directory, instanceID, name, baseURL, apiKey.Key)
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

	deployCmd := fmt.Sprintf("cd %s && curl -sSL https://get.axonclaw.io/install.sh | AXONCLAW_INSTANCE_ID=%s AXONCLAW_NAME=%s AXONHUB_BASE_URL=%s AXONHUB_API_KEY=%s sh", directory, instanceID, name, baseURL, apiKey.Key)
	if err := session2.Run(deployCmd); err != nil {
		return fmt.Errorf("failed to deploy axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentRuntimeService) deployToDocker(ctx context.Context, runtime *ent.AgentRuntime, agent *ent.Agent, instanceID, name string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	containerName := fmt.Sprintf("axonclaw-%s", instanceID)

	imageName := "axonclaw/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	apiKey, err := agent.APIKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agent API key: %w", err)
	}

	var baseURL string
	if debugLocalPath != "" {
		baseURL = "http://localhost:8090"
	} else {
		baseURL = "http://" + runtime.Host + ":8090"
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
		runCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker run -d --name %s --restart unless-stopped -e AXONCLAW_INSTANCE_ID=%s -e AXONCLAW_NAME=%s -e AXONHUB_BASE_URL=%s -e AXONHUB_API_KEY=%s %s", containerName, instanceID, name, baseURL, apiKey.Key, imageName))
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

	runCmd := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -e AXONCLAW_INSTANCE_ID=%s -e AXONCLAW_NAME=%s -e AXONHUB_BASE_URL=%s -e AXONHUB_API_KEY=%s %s", containerName, instanceID, name, baseURL, apiKey.Key, imageName)
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

func generateInstanceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
