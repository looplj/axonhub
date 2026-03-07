package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/agentmessage"
	"github.com/looplj/axonhub/internal/ent/messagechannelagentinstance"
	"github.com/looplj/axonhub/internal/ent/messagechannelbindingrequest"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

type MessageRouting struct {
	db      *ent.Client
	channel *ent.MessageChannel
}

func (r *MessageRouting) HandleInbound(ctx context.Context, msg InboundMessage) error {
	ctx = authz.WithSystemBypass(ctx, "system-message-gateway-handle-message")

	log.Debug(ctx, "processing inbound message",
		log.Int("channel_id", r.channel.ID),
		log.String("sender_id", msg.SenderID),
		log.String("chat_id", msg.ChatID),
		log.String("chat_type", string(msg.ChatType)),
		log.Bool("mentioned", msg.Mentioned),
		log.String("content_preview", truncate(msg.Content, 80)))

	if msg.ChatType == objects.MessageChatTypeGroup && !msg.Mentioned {
		log.Debug(ctx, "group message without mention, skipping",
			log.Int("channel_id", r.channel.ID),
			log.String("chat_id", msg.ChatID))

		return nil
	}

	if !r.isSenderAllowed(msg.SenderID) {
		log.Debug(ctx, "sender not allowed by channel",
			log.Int("channel_id", r.channel.ID),
			log.String("sender_id", msg.SenderID))

		return nil
	}

	if r.hasExcludedKeyword(msg.Content) {
		log.Debug(ctx, "message contains excluded keyword",
			log.Int("channel_id", r.channel.ID))

		return nil
	}

	if matched := r.tryMatchPairCode(ctx, msg); matched {
		log.Info(ctx, "pair code matched and binding created",
			log.Int("channel_id", r.channel.ID),
			log.String("chat_id", msg.ChatID))

		return nil
	}

	log.Debug(ctx, "inbound message received",
		log.Int("channel_id", r.channel.ID),
		log.String("sender_id", msg.SenderID),
		log.String("chat_id", msg.ChatID),
		log.String("chat_type", string(msg.ChatType)),
		log.String("content_preview", truncate(msg.Content, 80)))

	return r.routeToAgent(ctx, msg)
}

func (r *MessageRouting) isSenderAllowed(senderID string) bool {
	settings := r.channel.Settings

	var allowFrom []string
	if settings.Feishu != nil {
		allowFrom = settings.Feishu.AllowFrom
	}

	if len(allowFrom) == 0 {
		return true
	}

	for _, allowed := range allowFrom {
		trimmed := strings.TrimPrefix(allowed, "@")
		if senderID == allowed || senderID == trimmed {
			return true
		}
	}

	return false
}

func (r *MessageRouting) hasExcludedKeyword(content string) bool {
	settings := r.channel.Settings

	var excludeKeywords []string
	if settings.Feishu != nil {
		excludeKeywords = settings.Feishu.ExcludeKeywords
	}

	for _, keyword := range excludeKeywords {
		if keyword != "" && strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

func (r *MessageRouting) routeToAgent(ctx context.Context, msg InboundMessage) error {
	bindings, err := r.db.MessageChannelAgentInstance.Query().
		Where(
			messagechannelagentinstance.MessageChannelIDEQ(r.channel.ID),
			messagechannelagentinstance.EnabledEQ(true),
		).
		WithAgentInstance(func(q *ent.AgentInstanceQuery) {
			q.Where(agentinstance.StatusEQ(agentinstance.StatusRunning))
		}).
		All(ctx)
	if err != nil {
		log.Error(ctx, "failed to query agent bindings",
			log.Int("channel_id", r.channel.ID),
			log.Cause(err))

		return err
	}

	if len(bindings) == 0 {
		log.Debug(ctx, "no agent bindings for channel",
			log.Int("channel_id", r.channel.ID))

		return nil
	}

	log.Debug(ctx, "routing message to agents",
		log.Int("channel_id", r.channel.ID),
		log.String("sender_id", msg.SenderID),
		log.String("chat_id", msg.ChatID),
		log.Int("binding_count", len(bindings)))

	incomingTargetType := objects.MessageChatTypeDM
	if msg.ChatType == "group" {
		incomingTargetType = objects.MessageChatTypeGroup
	}

	for _, binding := range bindings {
		agentInstance := binding.Edges.AgentInstance
		if agentInstance == nil {
			log.Debug(ctx, "binding has no agent instance",
				log.Int("binding_id", binding.ID))

			continue
		}

		if binding.Config.ChatType != "" && binding.Config.ChatID != "" {
			if binding.Config.ChatType != incomingTargetType || binding.Config.ChatID != msg.ChatID {
				log.Debug(ctx, "binding target mismatch, skipping",
					log.Int("binding_id", binding.ID),
					log.Int("agent_instance_id", agentInstance.ID),
					log.String("binding_target_type", string(binding.Config.ChatType)),
					log.String("binding_target_id", binding.Config.ChatID),
					log.String("incoming_target_type", string(incomingTargetType)),
					log.String("incoming_chat_id", msg.ChatID))

				continue
			}
		}

		if !isBindingSenderAllowed(binding, msg.SenderID) {
			log.Debug(ctx, "sender not allowed for binding",
				log.Int("binding_id", binding.ID),
				log.Int("agent_instance_id", agentInstance.ID),
				log.String("sender_id", msg.SenderID))

			continue
		}

		if hasBindingExcludedKeyword(binding, msg.Content) {
			log.Debug(ctx, "message excluded by keyword",
				log.Int("binding_id", binding.ID),
				log.Int("agent_instance_id", agentInstance.ID))

			continue
		}

		log.Debug(ctx, "creating agent message",
			log.Int("channel_id", r.channel.ID),
			log.Int("agent_instance_id", agentInstance.ID),
			log.String("sender_id", msg.SenderID))

		agentMsg, err := r.createAgentMessage(ctx, agentInstance.ID, msg)
		if err != nil {
			log.Error(ctx, "failed to create agent message",
				log.Int("channel_id", r.channel.ID),
				log.Int("agent_instance_id", agentInstance.ID),
				log.Cause(err))

			continue
		}

		log.Info(ctx, "routed message to agent",
			log.Int("channel_id", r.channel.ID),
			log.Int("agent_instance_id", agentInstance.ID),
			log.Int("message_id", agentMsg.ID))
	}

	return nil
}

func (r *MessageRouting) createAgentMessage(ctx context.Context, agentInstanceID int, msg InboundMessage) (*ent.AgentMessage, error) {
	agent, err := r.db.AgentInstance.Query().
		Where(agentinstance.IDEQ(agentInstanceID)).
		QueryAgent().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("query agent: %w", err)
	}

	contentData := map[string]any{
		"text":       msg.Content,
		"chat_id":    msg.ChatID,
		"chat_type":  msg.ChatType,
		"message_id": msg.MessageID,
	}
	contentBytes, _ := json.Marshal(contentData)
	msgContent := objects.JSONRawMessage(contentBytes)

	var lastSeq int64

	lastMsg, err := r.db.AgentMessage.Query().
		Where(agentmessage.AgentIDEQ(agent.ID)).
		Order(ent.Desc(agentmessage.FieldSequence)).
		First(ctx)
	if err == nil && lastMsg != nil {
		lastSeq = lastMsg.Sequence
	}

	creator := r.db.AgentMessage.Create().
		SetProjectID(agent.ProjectID).
		SetAgentID(agent.ID).
		SetAgentInstanceID(agentInstanceID).
		SetDirection(agentmessage.DirectionToAgent).
		SetSenderType(agentmessage.SenderTypeMessageChannel).
		SetSenderID(r.channel.ID).
		SetType(agentmessage.TypeChat).
		SetContent(msgContent).
		SetStatus(agentmessage.StatusPending).
		SetSequence(lastSeq + 1)
	if msg.MessageID != "" {
		creator = creator.SetExternalMessageID(msg.MessageID)
	}

	agentMsg, err := creator.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	log.Debug(ctx, "agent message created",
		log.Int("message_id", agentMsg.ID),
		log.Int("agent_id", agent.ID),
		log.Int("agent_instance_id", agentInstanceID),
		log.Int64("sequence", agentMsg.Sequence))

	return agentMsg, nil
}

func (r *MessageRouting) tryMatchPairCode(ctx context.Context, msg InboundMessage) bool {
	content := strings.TrimSpace(msg.Content)
	if len(content) != 9 {
		log.Debug(ctx, "invalid pair code length", log.String("pair_code", content))
		return false
	}

	if !isPairCodeFormat(content) {
		log.Debug(ctx, "invalid pair code format", log.String("pair_code", content))
		return false
	}

	req, err := r.db.MessageChannelBindingRequest.Query().
		Where(
			messagechannelbindingrequest.PairCodeEQ(content),
			messagechannelbindingrequest.StatusEQ(messagechannelbindingrequest.StatusPending),
			messagechannelbindingrequest.MessageChannelIDEQ(r.channel.ID),
		).
		Only(ctx)
	if err != nil {
		log.Debug(ctx, "pair code not found or already used",
			log.String("pair_code", content),
			log.Cause(err))

		return false
	}

	if req.ExpiresAt.Before(time.Now()) {
		log.Debug(ctx, "pair code expired",
			log.String("pair_code", content))

		return false
	}

	config := objects.MessageChannelAgentInstanceBinding{
		ChatType: msg.ChatType,
		ChatID:   msg.ChatID,
	}

	existing, err := r.db.MessageChannelAgentInstance.Query().
		Where(
			messagechannelagentinstance.MessageChannelIDEQ(r.channel.ID),
			messagechannelagentinstance.AgentInstanceIDEQ(req.AgentInstanceID),
		).
		Only(ctx)
	if err == nil && existing != nil {
		_, err = r.db.MessageChannelAgentInstance.UpdateOneID(existing.ID).
			SetConfig(config).
			Save(ctx)
	} else {
		_, err = r.db.MessageChannelAgentInstance.Create().
			SetMessageChannelID(r.channel.ID).
			SetAgentInstanceID(req.AgentInstanceID).
			SetEnabled(true).
			SetConfig(config).
			Save(ctx)
	}

	if err != nil {
		log.Error(ctx, "failed to create or update binding",
			log.Int("agent_instance_id", req.AgentInstanceID),
			log.Cause(err))

		return false
	}

	_, err = r.db.MessageChannelBindingRequest.UpdateOneID(req.ID).
		SetStatus(messagechannelbindingrequest.StatusApproved).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "failed to update binding request status",
			log.Int("request_id", req.ID),
			log.Cause(err))
	}

	log.Info(ctx, "binding created via pair code",
		log.Int("channel_id", r.channel.ID),
		log.Int("agent_instance_id", req.AgentInstanceID),
		log.String("chat_id", msg.ChatID),
		log.String("target_type", string(msg.ChatType)))

	return true
}

func isBindingSenderAllowed(binding *ent.MessageChannelAgentInstance, senderID string) bool {
	config := binding.Config
	if len(config.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range config.AllowFrom {
		trimmed := strings.TrimPrefix(allowed, "@")
		if senderID == allowed || senderID == trimmed {
			return true
		}
	}

	return false
}

func hasBindingExcludedKeyword(binding *ent.MessageChannelAgentInstance, content string) bool {
	for _, keyword := range binding.Config.ExcludeKeywords {
		if keyword != "" && strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

var pairCodeRegex = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)

func isPairCodeFormat(s string) bool {
	return pairCodeRegex.MatchString(s)
}
