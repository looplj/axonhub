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
	"github.com/looplj/axonhub/internal/ent/agenthost"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/scopes"
)

var (
	debugLocalPath   = os.Getenv("AXONHUB_DEBUG_AXONCLAW_PATH")
	debugDockerImage = os.Getenv("AXONHUB_DEBUG_AXONCLAW_IMAGE")
)

func overrideEnv(base []string, key, value string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return base
	}

	prefix := key + "="

	out := make([]string, 0, len(base)+1)
	for _, item := range base {
		if strings.HasPrefix(item, prefix) {
			continue
		}

		out = append(out, item)
	}

	out = append(out, prefix+value)

	return out
}

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
	HostID         int
	Name           string
	AxonhubBaseURL string
}

// instanceDirectory returns the computed working directory for a vm/local instance.
// Format: {host_directory}/{agent_name}/{instance_name}.
func instanceDirectory(hostDir, agentName, instanceName string) string {
	return fmt.Sprintf("%s/%s/%s", hostDir, agentName, instanceName)
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

	host, err := svc.db.AgentHost.Query().Where(agenthost.IDEQ(input.HostID)).Only(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load host: %v", err),
		}, nil
	}

	if err := validateDeployInput(input, host); err != nil {
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

		instance, err = client.AgentInstance.Create().
			SetProjectID(entity.ProjectID).
			SetAgentID(input.AgentID).
			SetHostID(input.HostID).
			SetName(input.Name).
			SetStatus(agentinstance.StatusPending).
			SetAxonhubBaseURL(input.AxonhubBaseURL).
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

	err = svc.executeDeployment(ctx, host, instance, apiKey, entity.Name)
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

func (svc *AgentDeployService) executeDeployment(ctx context.Context, host *ent.AgentHost, instance *ent.AgentInstance, apiKey *ent.APIKey, agentName string) error {
	var err error

	baseURL := instance.AxonhubBaseURL
	dir := instanceDirectory(host.Directory, agentName, instance.Name)

	switch host.Type {
	case agenthost.TypeVM:
		err = svc.deployToVM(ctx, host, apiKey, instance.Name, dir, baseURL)
	case agenthost.TypeDocker:
		err = svc.deployToDocker(ctx, host, apiKey, instance.Name, baseURL)
	case agenthost.TypeLocal:
		err = svc.deployToLocal(ctx, host, apiKey, instance.Name, dir, baseURL)
	}

	if err != nil {
		_, _ = svc.db.AgentInstance.UpdateOneID(instance.ID).
			SetStatus(agentinstance.StatusError).
			Save(ctx)

		return fmt.Errorf("failed to deploy to host %s: %w", host.Type, err)
	}

	_, _ = svc.db.AgentInstance.UpdateOneID(instance.ID).
		SetStatus(agentinstance.StatusRunning).
		Save(ctx)

	return nil
}

func validateDeployInput(input DeployAxonclawInput, host *ent.AgentHost) error {
	if input.AgentID <= 0 {
		return fmt.Errorf("agent ID is required")
	}

	if input.HostID <= 0 {
		return fmt.Errorf("host ID is required")
	}

	if input.Name == "" {
		return fmt.Errorf("name is required")
	}

	if (host.Type == agenthost.TypeVM || host.Type == agenthost.TypeLocal) && strings.TrimSpace(host.Directory) == "" {
		return fmt.Errorf("host directory is required for %s host type", host.Type)
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
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlStart, nil)
}

func (svc *AgentDeployService) StopAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlStop, nil)
}

func (svc *AgentDeployService) RestartAxonclawInstance(ctx context.Context, instanceID int) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlRestart, nil)
}

func (svc *AgentDeployService) RedeployAxonclawInstance(ctx context.Context, instanceID int, axonhubBaseUrl *string) (*ControlAxonclawInstanceResult, error) {
	return svc.controlAxonclawInstance(ctx, instanceID, axonclawControlRedeploy, axonhubBaseUrl)
}

//nolint:nilerr // ignore nil error, it's handled in the function body.
func (svc *AgentDeployService) controlAxonclawInstance(ctx context.Context, instanceID int, action axonclawControlAction, axonhubBaseUrl *string) (*ControlAxonclawInstanceResult, error) {
	client := svc.entFromContext(ctx)

	instance, err := client.AgentInstance.Query().
		Where(agentinstance.IDEQ(instanceID)).
		WithHost().
		WithAgent().
		Only(ctx)
	if err != nil {
		return &ControlAxonclawInstanceResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load instance: %v", err),
		}, nil
	}

	if instance.Edges.Host == nil {
		return &ControlAxonclawInstanceResult{
			Success:  false,
			Error:    "instance is not bound to a host",
			Instance: instance,
		}, nil
	}

	if instance.Edges.Agent == nil {
		return &ControlAxonclawInstanceResult{
			Success:  false,
			Error:    "instance agent not found",
			Instance: instance,
		}, nil
	}

	host := instance.Edges.Host
	agentName := instance.Edges.Agent.Name
	dir := instanceDirectory(host.Directory, agentName, instance.Name)
	baseURL := instance.AxonhubBaseURL

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
		actionErr = svc.stopAxonclaw(ctx, host, instance.Name, dir)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusStopped).Save(ctx)
		}
	case axonclawControlStart:
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusPending).Save(ctx)

		actionErr = svc.startAxonclaw(ctx, host, apiKey, instance.Name, dir, baseURL)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusRunning).Save(ctx)
		}
	case axonclawControlRestart:
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusPending).Save(ctx)

		actionErr = svc.restartAxonclaw(ctx, host, apiKey, instance.Name, dir, baseURL)
		if actionErr == nil {
			_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusRunning).Save(ctx)
		}
	case axonclawControlRedeploy:
		_, _ = client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusPending).Save(ctx)

		redeployBaseURL := baseURL
		if axonhubBaseUrl != nil && *axonhubBaseUrl != "" {
			redeployBaseURL = *axonhubBaseUrl
		}

		actionErr = svc.redeployAxonclaw(ctx, host, apiKey, instance.Name, dir, redeployBaseURL)
		if actionErr == nil {
			update := client.AgentInstance.UpdateOneID(instance.ID).SetStatus(agentinstance.StatusRunning)
			if axonhubBaseUrl != nil && *axonhubBaseUrl != "" {
				update = update.SetAxonhubBaseURL(redeployBaseURL)
			}

			_, _ = update.Save(ctx)
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

func (svc *AgentDeployService) stopAxonclaw(ctx context.Context, runtime *ent.AgentHost, name, directory string) error {
	switch runtime.Type {
	case agenthost.TypeVM:
		return svc.vmStop(ctx, runtime, directory)
	case agenthost.TypeDocker:
		return svc.dockerStop(ctx, runtime, dockerContainerName(name))
	case agenthost.TypeLocal:
		return svc.localStop(ctx, directory)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func (svc *AgentDeployService) startAxonclaw(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	switch runtime.Type {
	case agenthost.TypeVM:
		return svc.vmStart(ctx, runtime, apiKey, name, directory, baseURL)
	case agenthost.TypeDocker:
		return svc.dockerStart(ctx, runtime, dockerContainerName(name))
	case agenthost.TypeLocal:
		return svc.localStart(ctx, apiKey, name, directory, baseURL)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func (svc *AgentDeployService) restartAxonclaw(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	switch runtime.Type {
	case agenthost.TypeVM:
		return svc.vmRestart(ctx, runtime, apiKey, name, directory, baseURL)
	case agenthost.TypeDocker:
		return svc.dockerRestart(ctx, runtime, dockerContainerName(name))
	case agenthost.TypeLocal:
		return svc.localRestart(ctx, apiKey, name, directory, baseURL)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func (svc *AgentDeployService) redeployAxonclaw(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	switch runtime.Type {
	case agenthost.TypeVM:
		_ = svc.vmStop(ctx, runtime, directory)
		if err := svc.vmInstallLatest(ctx, runtime, apiKey, name, directory, baseURL); err != nil {
			return err
		}

		return svc.vmStart(ctx, runtime, apiKey, name, directory, baseURL)
	case agenthost.TypeDocker:
		return svc.dockerRedeploy(ctx, runtime, apiKey, name, dockerContainerName(name), baseURL)
	case agenthost.TypeLocal:
		_ = svc.localStop(ctx, directory)
		if err := svc.localInstallLatest(ctx, apiKey, name, directory, baseURL); err != nil {
			return err
		}

		return svc.localStart(ctx, apiKey, name, directory, baseURL)
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtime.Type)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type DeployAxonClawByAgentInput struct {
	Name string
}

func (svc *AgentDeployService) DeployAxonClawByAgent(ctx context.Context, currentInst *ent.AgentInstance, input DeployAxonClawByAgentInput) (*DeployAxonclawResult, error) {
	if currentInst.AgentHostID == nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   "current instance is not bound to a host",
		}, nil
	}

	host, err := svc.db.AgentHost.Query().Where(agenthost.IDEQ(*currentInst.AgentHostID)).Only(ctx)
	if err != nil {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load host: %v", err),
		}, nil
	}

	if host.Status != agenthost.StatusActive {
		return &DeployAxonclawResult{
			Success: false,
			Error:   "host is not active",
		}, nil
	}

	if input.Name == "" {
		return &DeployAxonclawResult{
			Success: false,
			Error:   "name is required",
		}, nil
	}

	if (host.Type == agenthost.TypeVM || host.Type == agenthost.TypeLocal) && strings.TrimSpace(host.Directory) == "" {
		return &DeployAxonclawResult{
			Success: false,
			Error:   fmt.Sprintf("host directory is required for %s host type", host.Type),
		}, nil
	}

	baseURL := currentInst.AxonhubBaseURL

	entity, err := svc.db.Agent.Query().
		Where(agent.IDEQ(currentInst.AgentID)).
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

		apiKeyName := fmt.Sprintf("agent-instance:%d:%s", currentInst.AgentID, input.Name)

		generatedKey, err := GenerateAPIKey()
		if err != nil {
			return fmt.Errorf("failed to generate api key: %w", err)
		}

		apiKey, err = authz.RunWithSystemBypass(txCtx, "agent-deploy-axonclaw-api-key", func(bypassCtx context.Context) (*ent.APIKey, error) {
			return client.APIKey.Create().
				SetName(apiKeyName).
				SetKey(generatedKey).
				SetUserID(entity.CreatedByUserID).
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

		instance, err = client.AgentInstance.Create().
			SetProjectID(entity.ProjectID).
			SetAgentID(currentInst.AgentID).
			SetHostID(*currentInst.AgentHostID).
			SetName(input.Name).
			SetStatus(agentinstance.StatusPending).
			SetAxonhubBaseURL(baseURL).
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

	err = svc.executeDeployment(ctx, host, instance, apiKey, entity.Name)
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
