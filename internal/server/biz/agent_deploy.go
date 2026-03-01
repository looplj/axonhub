//nolint:gosec,nilerr // G204: Subprocess launched with variable.
package biz

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/fx"

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

type AgentDeployServiceParams struct {
	fx.In

	Ent *ent.Client
}

// AgentDeployService provides APIs for deploying and managing agent instances.
// This service handles deployment, start, stop, restart, and redeploy operations.
type AgentDeployService struct {
	*AbstractService
}

func NewAgentDeployService(params AgentDeployServiceParams) *AgentDeployService {
	return &AgentDeployService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
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

func (svc *AgentDeployService) DeployAxonclaw(ctx context.Context, input DeployAxonclawInput) (*DeployAxonclawResult, error) {
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

func (svc *AgentDeployService) executeDeployment(ctx context.Context, runtime *ent.AgentRuntime, instance *ent.AgentInstance, apiKey *ent.APIKey, input DeployAxonclawInput) error {
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

func (svc *AgentDeployService) StartAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlStart)
}

func (svc *AgentDeployService) StopAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlStop)
}

func (svc *AgentDeployService) RestartAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlRestart)
}

func (svc *AgentDeployService) RedeployAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlRedeploy)
}

//nolint:nilerr // ignore nil error, it's handled in the function body.
func (svc *AgentDeployService) controlAxonclawInstance(ctx context.Context, instanceID int, action axonclawControlAction) (*ControlAxonclawInstanceResult, error) {
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

func (svc *AgentDeployService) stopAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, name string, deployment objects.AgentInstanceDeployment) error {
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

func (svc *AgentDeployService) startAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name string, deployment objects.AgentInstanceDeployment) error {
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

func (svc *AgentDeployService) restartAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name string, deployment objects.AgentInstanceDeployment) error {
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

func (svc *AgentDeployService) redeployAxonclaw(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name string, deployment objects.AgentInstanceDeployment) error {
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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
