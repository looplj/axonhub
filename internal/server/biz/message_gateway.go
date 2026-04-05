package biz

import (
	"context"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/agentmessage"
	"github.com/looplj/axonhub/internal/ent/messagechannel"
	"github.com/looplj/axonhub/internal/ent/messagechannelagentinstance"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

const (
	channelCheckInterval = 30 * time.Second
)

type MessageGatewayParams struct {
	fx.In

	Ent *ent.Client
}

type MessageGateway struct {
	db *ent.Client

	mu       sync.RWMutex
	channels map[int]*channelRunner
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

type channelRunner struct {
	channel *ent.MessageChannel
	handler ChannelHandler
	routing *MessageRouting
	cancel  context.CancelFunc
}

func NewMessageGateway(params MessageGatewayParams) *MessageGateway {
	return &MessageGateway{
		db:       params.Ent,
		channels: make(map[int]*channelRunner),
	}
}

func (g *MessageGateway) Start(ctx context.Context) error {
	log.Info(ctx, "MessageGateway starting")

	runCtx, cancel := context.WithCancel(authz.WithSystemBypass(context.Background(), "system-message-gateway"))

	g.mu.Lock()
	g.cancel = cancel
	g.mu.Unlock()

	g.wg.Add(1)

	go g.runChannelWatcher(runCtx)

	log.Info(ctx, "MessageGateway started")

	return nil
}

func (g *MessageGateway) Stop(ctx context.Context) error {
	log.Info(ctx, "MessageGateway stopping")

	g.mu.Lock()
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}

	channels := g.channels
	g.mu.Unlock()

	for _, runner := range channels {
		if runner.cancel != nil {
			runner.cancel()
		}

		if runner.handler != nil {
			runner.handler.Stop()
		}
	}

	g.wg.Wait()

	log.Info(ctx, "MessageGateway stopped")

	return nil
}

func (g *MessageGateway) runChannelWatcher(ctx context.Context) {
	defer g.wg.Done()

	log.Debug(ctx, "channel watcher started")

	ticker := time.NewTicker(channelCheckInterval)
	defer ticker.Stop()

	g.syncChannels(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Debug(ctx, "channel watcher stopped")
			return
		case <-ticker.C:
			log.Debug(ctx, "channel watcher tick, syncing channels")
			g.syncChannels(ctx)
		}
	}
}

func (g *MessageGateway) syncChannels(ctx context.Context) {
	channels, err := g.db.MessageChannel.Query().
		Where(messagechannel.StatusEQ(messagechannel.StatusEnabled)).
		WithAgentInstanceBindings(func(q *ent.MessageChannelAgentInstanceQuery) {
			q.Where(messagechannelagentinstance.EnabledEQ(true)).
				WithAgentInstance(func(q *ent.AgentInstanceQuery) {
					q.Where(agentinstance.StatusEQ(agentinstance.StatusRunning))
				})
		}).
		All(ctx)
	if err != nil {
		log.Error(ctx, "failed to query message channels", log.Cause(err))
		return
	}

	log.Debug(ctx, "syncing message channels",
		log.Int("channel_count", len(channels)))

	activeIDs := make(map[int]struct{})
	for _, ch := range channels {
		activeIDs[ch.ID] = struct{}{}
		bindingCount := len(ch.Edges.AgentInstanceBindings)
		log.Debug(ctx, "checking channel",
			log.Int("channel_id", ch.ID),
			log.String("name", ch.Name),
			log.String("type", string(ch.Type)),
			log.Int("binding_count", bindingCount))
		g.startChannelIfNeeded(ctx, ch)
	}

	g.mu.RLock()
	currentChannelCount := len(g.channels)
	g.mu.RUnlock()

	log.Debug(ctx, "channel sync status",
		log.Int("active_channels", len(activeIDs)),
		log.Int("running_channels", currentChannelCount))

	g.mu.RLock()

	for id := range g.channels {
		if _, ok := activeIDs[id]; !ok {
			g.mu.RUnlock()
			log.Debug(ctx, "stopping inactive channel", log.Int("channel_id", id))
			g.stopChannel(ctx, id)
			g.mu.RLock()
		}
	}

	g.mu.RUnlock()
}

func (g *MessageGateway) startChannelIfNeeded(ctx context.Context, ch *ent.MessageChannel) {
	g.mu.RLock()
	_, exists := g.channels[ch.ID]
	g.mu.RUnlock()

	if exists {
		log.Debug(ctx, "channel already running", log.Int("channel_id", ch.ID))
		return
	}

	routing := &MessageRouting{
		db:      g.db,
		channel: ch,
	}

	handler, err := createChannelHandler(ctx, ch, routing)
	if err != nil {
		log.Error(ctx, "failed to create channel handler",
			log.Int("channel_id", ch.ID),
			log.String("type", string(ch.Type)),
			log.Cause(err))

		return
	}

	if handler == nil {
		log.Debug(ctx, "no handler for channel type",
			log.Int("channel_id", ch.ID),
			log.String("type", string(ch.Type)))

		return
	}

	runCtx, cancel := context.WithCancel(ctx)

	if err := handler.Start(runCtx); err != nil {
		cancel()
		log.Error(ctx, "failed to start channel handler",
			log.Int("channel_id", ch.ID),
			log.String("type", string(ch.Type)),
			log.Cause(err))

		return
	}

	runner := &channelRunner{
		channel: ch,
		handler: handler,
		routing: routing,
		cancel:  cancel,
	}

	go runner.watchAgentMessages(runCtx)

	g.mu.Lock()
	g.channels[ch.ID] = runner
	g.mu.Unlock()

	log.Info(ctx, "message channel runner started",
		log.Int("channel_id", ch.ID),
		log.String("name", ch.Name),
		log.String("type", string(ch.Type)))
}

func (g *MessageGateway) stopChannel(ctx context.Context, id int) {
	g.mu.Lock()

	runner, ok := g.channels[id]
	if ok {
		delete(g.channels, id)
	}
	g.mu.Unlock()

	if ok && runner != nil {
		if runner.handler != nil {
			runner.handler.Stop()
		}

		if runner.cancel != nil {
			runner.cancel()
		}

		log.Info(ctx, "message channel runner stopped",
			log.Int("channel_id", id))
	}
}

func (r *channelRunner) watchAgentMessages(ctx context.Context) {
	log.Debug(ctx, "message channel runner starting agent message watcher",
		log.Int("channel_id", r.channel.ID))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug(ctx, "message channel runner agent message watcher stopped",
				log.Int("channel_id", r.channel.ID))

			return
		case <-ticker.C:
			r.processPendingAgentMessages(ctx)
		}
	}
}

func (r *channelRunner) processPendingAgentMessages(ctx context.Context) {
	bindings, err := r.routing.db.MessageChannelAgentInstance.Query().
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

		return
	}

	if len(bindings) == 0 {
		return
	}

	bindingByInstanceID := make(map[int]*ent.MessageChannelAgentInstance)

	var agentInstanceIDs []int

	for _, binding := range bindings {
		if binding.Edges.AgentInstance != nil {
			agentInstanceIDs = append(agentInstanceIDs, binding.Edges.AgentInstance.ID)
			bindingByInstanceID[binding.Edges.AgentInstance.ID] = binding
		}
	}

	if len(agentInstanceIDs) == 0 {
		return
	}

	messages, err := r.routing.db.AgentMessage.Query().
		Where(
			agentmessage.DirectionEQ(agentmessage.DirectionToUser),
			agentmessage.StatusEQ(agentmessage.StatusPending),
			agentmessage.AgentInstanceIDIn(agentInstanceIDs...),
		).
		All(ctx)
	if err != nil {
		log.Error(ctx, "failed to query pending agent messages",
			log.Int("channel_id", r.channel.ID),
			log.Cause(err))

		return
	}

	if len(messages) > 0 {
		log.Debug(ctx, "processing pending agent messages",
			log.Int("channel_id", r.channel.ID),
			log.Int("message_count", len(messages)))
	}

	for _, msg := range messages {
		log.Debug(ctx, "processing agent message",
			log.Int("channel_id", r.channel.ID),
			log.Int("message_id", msg.ID))

		content, err := extractMessageContent(msg.Content)
		if err != nil {
			log.Error(ctx, "failed to extract message content",
				log.Int("message_id", msg.ID),
				log.Cause(err))

			continue
		}

		replyToMessageID, replyToChatID := r.resolveReplyTarget(ctx, msg)
		if msg.ReplyToMessageID != nil && replyToMessageID == "" {
			log.Debug(ctx, "skip sending message: reply_to_message_id set but reply target not resolvable for this channel",
				log.Int("message_id", msg.ID),
				log.Int("channel_id", r.channel.ID),
				log.Any("reply_to_message_id", msg.ReplyToMessageID))

			continue
		}

		var chatID string
		if binding, ok := bindingByInstanceID[msg.AgentInstanceID]; ok && binding.Config.ChatID != "" {
			chatID = binding.Config.ChatID
		} else if replyToChatID != "" {
			chatID = replyToChatID
		}

		if chatID == "" {
			log.Warn(ctx, "no chat_id resolved for agent message",
				log.Int("message_id", msg.ID),
				log.Int("agent_instance_id", msg.AgentInstanceID))

			continue
		}

		log.Debug(ctx, "sending message via channel handler",
			log.Int("message_id", msg.ID),
			log.String("chat_id", chatID),
			log.String("reply_to", replyToMessageID),
			log.String("content_preview", truncate(content, 80)))

		if err := r.handler.SendMessage(ctx, OutboundMessage{
			ChatID:           chatID,
			Content:          content,
			ReplyToMessageID: replyToMessageID,
		}); err != nil {
			log.Error(ctx, "failed to send message",
				log.Int("message_id", msg.ID),
				log.Cause(err))

			continue
		}

		_, err = r.routing.db.AgentMessage.UpdateOneID(msg.ID).
			SetStatus(agentmessage.StatusAcked).
			Save(ctx)
		if err != nil {
			log.Error(ctx, "failed to ack agent message",
				log.Int("message_id", msg.ID),
				log.Cause(err))
		} else {
			log.Debug(ctx, "agent message sent and acked",
				log.Int("message_id", msg.ID))
		}
	}
}

func extractMessageContent(content objects.JSONRawMessage) (string, error) {
	text := ExtractTextFromContent(content)
	if text == "" {
		return string(content), nil
	}

	return text, nil
}

func (r *channelRunner) resolveReplyTarget(ctx context.Context, msg *ent.AgentMessage) (replyToExternalMessageID string, replyToChatID string) {
	if msg.ReplyToMessageID == nil || *msg.ReplyToMessageID <= 0 {
		return "", ""
	}

	replyTo, err := r.routing.db.AgentMessage.Query().
		Where(
			agentmessage.IDEQ(*msg.ReplyToMessageID),
			agentmessage.DirectionEQ(agentmessage.DirectionToAgent),
			agentmessage.SenderTypeEQ(agentmessage.SenderTypeMessageChannel),
			agentmessage.SenderIDEQ(r.channel.ID),
			agentmessage.ExternalMessageIDNotNil(),
		).
		Only(ctx)
	if err != nil || replyTo == nil || replyTo.ExternalMessageID == nil || *replyTo.ExternalMessageID == "" {
		log.Debug(ctx, "reply target not found or missing external_message_id",
			log.Int("message_id", msg.ID),
			log.Int("channel_id", r.channel.ID),
			log.Any("reply_to_message_id", msg.ReplyToMessageID),
			log.Cause(err))

		return "", ""
	}

	return *replyTo.ExternalMessageID, ExtractChatIDFromContent(replyTo.Content)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	return s[:maxLen-3] + "..."
}
