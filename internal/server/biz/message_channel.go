package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/messagechannel"
	"github.com/looplj/axonhub/internal/ent/messagechannelagentinstance"
	"github.com/looplj/axonhub/internal/ent/messagechannelbindingrequest"
	"github.com/looplj/axonhub/internal/objects"
)

type MessageChannelServiceParams struct {
	fx.In

	Ent *ent.Client
}

func NewMessageChannelService(params MessageChannelServiceParams) *MessageChannelService {
	return &MessageChannelService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

type MessageChannelService struct {
	*AbstractService
}

type BindingActionType string

const (
	BindingActionTypeCreate BindingActionType = "create"
	BindingActionTypeUpdate BindingActionType = "update"
	BindingActionTypeDelete BindingActionType = "delete"
	BindingActionTypeSkip   BindingActionType = "skip"
)

type BindingChangeAction struct {
	Type            BindingActionType
	AgentInstanceID int
	Enabled         bool
	Config          objects.MessageChannelAgentInstanceBinding
	ExistingBinding *ent.MessageChannelAgentInstance
}

func (svc *MessageChannelService) CreateMessageChannel(ctx context.Context, input ent.CreateMessageChannelInput) (*ent.MessageChannel, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	channel, err := svc.entFromContext(ctx).MessageChannel.Create().
		SetProjectID(projectID).
		SetInput(input).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create message channel: %w", err)
	}

	return channel, nil
}

type CreateBindingRequestInput struct {
	MessageChannelID int
	AgentInstanceID  int
	Type             messagechannelbindingrequest.Type
}

func (svc *MessageChannelService) CreateBindingRequest(ctx context.Context, input CreateBindingRequestInput) (*ent.MessageChannelBindingRequest, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	db := svc.entFromContext(ctx)

	channel, err := db.MessageChannel.Query().
		Where(
			messagechannel.IDEQ(input.MessageChannelID),
			messagechannel.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("message channel not found or not in project")
	}

	pairCode, err := generatePairCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate pair code: %w", err)
	}

	expiresAt := time.Now().Add(1 * time.Hour)

	req, err := db.MessageChannelBindingRequest.Create().
		SetMessageChannelID(channel.ID).
		SetAgentInstanceID(input.AgentInstanceID).
		SetType(input.Type).
		SetPairCode(pairCode).
		SetStatus(messagechannelbindingrequest.StatusPending).
		SetExpiresAt(expiresAt).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create binding request: %w", err)
	}

	return req, nil
}

func generatePairCode() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	code := strings.ToUpper(hex.EncodeToString(bytes))

	return code[:4] + "-" + code[4:], nil
}

func (svc *MessageChannelService) UpdateMessageChannel(ctx context.Context, id int, input ent.UpdateMessageChannelInput) (*ent.MessageChannel, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	updateBuilder := svc.entFromContext(ctx).MessageChannel.Update().
		Where(
			messagechannel.IDEQ(id),
			messagechannel.ProjectIDEQ(projectID),
		).
		SetInput(input)

	n, err := updateBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update message channel: %w", err)
	}

	if n == 0 {
		return nil, fmt.Errorf("message channel not found or not in project")
	}

	channel, err := svc.entFromContext(ctx).MessageChannel.Query().
		Where(
			messagechannel.IDEQ(id),
			messagechannel.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated message channel: %w", err)
	}

	return channel, nil
}

func (svc *MessageChannelService) DeleteMessageChannel(ctx context.Context, id int) error {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return fmt.Errorf("project id not found in context")
	}

	err := svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		client := svc.entFromContext(txCtx)

		_, err := client.MessageChannelAgentInstance.Delete().
			Where(messagechannelagentinstance.MessageChannelIDEQ(id)).
			Exec(txCtx)
		if err != nil {
			return fmt.Errorf("failed to delete message channel bindings: %w", err)
		}

		n, err := client.MessageChannel.Delete().
			Where(
				messagechannel.IDEQ(id),
				messagechannel.ProjectIDEQ(projectID),
			).
			Exec(txCtx)
		if err != nil {
			return fmt.Errorf("failed to delete message channel: %w", err)
		}

		if n == 0 {
			return fmt.Errorf("message channel not found or not in project")
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

type BatchSaveBindingInput struct {
	AgentInstanceID int
	Enabled         bool
	Config          objects.MessageChannelAgentInstanceBinding
}

func calculateBindingChanges(existingBindings []*ent.MessageChannelAgentInstance, inputs []*BatchSaveBindingInput) []BindingChangeAction {
	existingMap := make(map[int]*ent.MessageChannelAgentInstance, len(existingBindings))
	for _, b := range existingBindings {
		existingMap[b.AgentInstanceID] = b
	}

	inputSet := make(map[int]struct{}, len(inputs))

	var actions []BindingChangeAction

	for _, input := range inputs {
		agentInstanceID := input.AgentInstanceID
		inputSet[agentInstanceID] = struct{}{}

		existing, ok := existingMap[agentInstanceID]
		if !ok {
			actions = append(actions, BindingChangeAction{
				Type:            BindingActionTypeCreate,
				AgentInstanceID: agentInstanceID,
				Enabled:         input.Enabled,
				Config:          input.Config,
				ExistingBinding: nil,
			})
		} else {
			if existing.Enabled == input.Enabled &&
				existing.Config.Equals(&input.Config) {
				actions = append(actions, BindingChangeAction{
					Type:            BindingActionTypeSkip,
					AgentInstanceID: agentInstanceID,
					Enabled:         input.Enabled,
					Config:          input.Config,
					ExistingBinding: existing,
				})
			} else {
				actions = append(actions, BindingChangeAction{
					Type:            BindingActionTypeUpdate,
					AgentInstanceID: agentInstanceID,
					Enabled:         input.Enabled,
					Config:          input.Config,
					ExistingBinding: existing,
				})
			}
		}
	}

	for _, existing := range existingBindings {
		if _, ok := inputSet[existing.AgentInstanceID]; !ok {
			actions = append(actions, BindingChangeAction{
				Type:            BindingActionTypeDelete,
				AgentInstanceID: existing.AgentInstanceID,
				ExistingBinding: existing,
			})
		}
	}

	return actions
}

func (svc *MessageChannelService) BatchSaveMessageChannelBindings(ctx context.Context, messageChannelID int, bindings []*BatchSaveBindingInput) (*ent.MessageChannel, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	seenAgentInstanceIDs := make(map[int]struct{}, len(bindings))
	for _, b := range bindings {
		if _, ok := seenAgentInstanceIDs[b.AgentInstanceID]; ok {
			return nil, fmt.Errorf("duplicate binding input: agent_instance_id=%d", b.AgentInstanceID)
		}

		seenAgentInstanceIDs[b.AgentInstanceID] = struct{}{}
	}

	db := svc.entFromContext(ctx)

	channel, err := db.MessageChannel.Query().
		Where(
			messagechannel.IDEQ(messageChannelID),
			messagechannel.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("message channel not found or not in project")
	}

	existingBindings, err := db.MessageChannelAgentInstance.Query().
		Where(messagechannelagentinstance.MessageChannelIDEQ(messageChannelID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing bindings: %w", err)
	}

	actions := calculateBindingChanges(existingBindings, bindings)

	err = svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		client := svc.entFromContext(txCtx)

		for _, action := range actions {
			var err error

			switch action.Type {
			case BindingActionTypeSkip:
				continue

			case BindingActionTypeDelete:
				err = client.MessageChannelAgentInstance.DeleteOne(action.ExistingBinding).Exec(txCtx)
				if err != nil {
					return fmt.Errorf("failed to delete binding: %w", err)
				}

			case BindingActionTypeCreate:
				_, err = client.MessageChannelAgentInstance.Create().
					SetMessageChannelID(messageChannelID).
					SetAgentInstanceID(action.AgentInstanceID).
					SetEnabled(action.Enabled).
					SetConfig(action.Config).
					Save(txCtx)
				if err != nil {
					return fmt.Errorf("failed to create binding: %w", err)
				}

			case BindingActionTypeUpdate:
				_, err = client.MessageChannelAgentInstance.UpdateOneID(action.ExistingBinding.ID).
					SetEnabled(action.Enabled).
					SetConfig(action.Config).
					Save(txCtx)
				if err != nil {
					return fmt.Errorf("failed to update binding: %w", err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return channel, nil
}
