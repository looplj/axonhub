package biz

import (
	"context"
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

		if msg.ChatType == objects.MessageChatTypeGroup && !msg.Mentioned && !binding.Config.AllowWithoutMention {
			log.Debug(ctx, "group message without mention and binding requires mention, skipping",
				log.Int("binding_id", binding.ID),
				log.Int("agent_instance_id", agentInstance.ID),
				log.String("chat_id", msg.ChatID))

			continue
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

	msgContent := MarshalChatContent(msg.Content, msg.ChatID, string(msg.ChatType), msg.MessageID)

	input := CreateAgentMessageParams{
		ProjectID:       agent.ProjectID,
		AgentID:         agent.ID,
		AgentInstanceID: agentInstanceID,
		Direction:       agentmessage.DirectionToAgent,
		SenderType:      agentmessage.SenderTypeMessageChannel,
		SenderID:        &r.channel.ID,
		Type:            agentmessage.TypeChat,
		Content:         msgContent,
	}
	if msg.MessageID != "" {
		input.ExternalMessageID = &msg.MessageID
	}

	agentMsg, err := CreateAgentMessage(ctx, r.db, input)
	if err != nil {
		return nil, err
	}

	log.Debug(ctx, "agent message created",
		log.Int("message_id", agentMsg.ID),
		log.Int("agent_id", agent.ID),
		log.Int("agent_instance_id", agentInstanceID),
		log.Int64("sequence", agentMsg.Sequence))

	return agentMsg, nil
}

func (r *MessageRouting) tryMatchPairCode(ctx context.Context, msg InboundMessage) bool {
	if msg.ChatType != objects.MessageChatTypeDM && !msg.Mentioned {
		return false
	}
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

	r.sendPairingNotification(ctx, req.AgentInstanceID, msg)

	return true
}

func (r *MessageRouting) sendPairingNotification(ctx context.Context, agentInstanceID int, msg InboundMessage) {
	agent, err := r.db.AgentInstance.Query().
		Where(agentinstance.IDEQ(agentInstanceID)).
		QueryAgent().
		Only(ctx)
	if err != nil {
		log.Error(ctx, "failed to query agent for pairing notification",
			log.Int("agent_instance_id", agentInstanceID),
			log.Cause(err))

		return
	}

	r.sendPairingNotificationToAgent(ctx, agent, agentInstanceID, msg)
	r.sendPairingSuccessToUser(ctx, agent, agentInstanceID, msg)
}

func (r *MessageRouting) sendPairingNotificationToAgent(ctx context.Context, agent *ent.Agent, agentInstanceID int, msg InboundMessage) {
	msgContent := MarshalPairingSuccessContent(msg.ChatID, string(msg.ChatType), msg.MessageID, r.channel.ID)

	input := CreateAgentMessageParams{
		ProjectID:       agent.ProjectID,
		AgentID:         agent.ID,
		AgentInstanceID: agentInstanceID,
		Direction:       agentmessage.DirectionToAgent,
		SenderType:      agentmessage.SenderTypeSystem,
		Type:            agentmessage.TypeSystemEvent,
		Content:         msgContent,
	}

	_, err := CreateAgentMessage(ctx, r.db, input)
	if err != nil {
		log.Error(ctx, "failed to create pairing notification message",
			log.Int("agent_instance_id", agentInstanceID),
			log.Cause(err))

		return
	}

	log.Info(ctx, "pairing notification sent to agent",
		log.Int("agent_instance_id", agentInstanceID),
		log.Int("agent_id", agent.ID))
}

func (r *MessageRouting) sendPairingSuccessToUser(ctx context.Context, agent *ent.Agent, agentInstanceID int, msg InboundMessage) {
	content := MarshalTextContent("✅ Pairing successful! You can now chat with the Agent.")

	input := CreateAgentMessageParams{
		ProjectID:       agent.ProjectID,
		AgentID:         agent.ID,
		AgentInstanceID: agentInstanceID,
		Direction:       agentmessage.DirectionToUser,
		SenderType:      agentmessage.SenderTypeSystem,
		Type:            agentmessage.TypeChat,
		Content:         content,
	}

	_, err := CreateAgentMessage(ctx, r.db, input)
	if err != nil {
		log.Error(ctx, "failed to create pairing success message for user",
			log.Int("agent_instance_id", agentInstanceID),
			log.Cause(err))

		return
	}

	log.Info(ctx, "pairing success message created for user",
		log.Int("agent_instance_id", agentInstanceID),
		log.String("chat_id", msg.ChatID))
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
