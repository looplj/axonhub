package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/messagechannel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// InboundMessage is a platform-agnostic inbound message produced by ChannelHandler.
type InboundMessage struct {
	SenderID  string
	ChatID    string
	ChatType  objects.MessageChatType
	MessageID string
	Content   string
	Mentioned bool
}

// OutboundMessage is a platform-agnostic outbound message to be sent by ChannelHandler.
type OutboundMessage struct {
	ChatID           string
	Content          string
	ReplyToMessageID string
}

// ChannelHandler abstracts the channel-specific behavior for a message channel type.
// Each channel type (feishu, slack, dingtalk, etc.) implements this interface.
type ChannelHandler interface {
	// Start initiates the channel connection (e.g., websocket) and begins processing messages.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the channel connection.
	Stop()

	// SendMessage sends a message to the specified target (e.g., chat_id).
	// replyToMessageID is the platform message ID to reply to (may be empty for direct send).
	SendMessage(ctx context.Context, msg OutboundMessage) error
}

// ChannelHandlerFactory creates a ChannelHandler for a given channel.
type ChannelHandlerFactory func(ctx context.Context, ch *ent.MessageChannel, routing *MessageRouting) (ChannelHandler, error)

// channelHandlerRegistry maps channel types to their factory functions.
var channelHandlerRegistry = map[messagechannel.Type]ChannelHandlerFactory{}

// RegisterChannelHandler registers a factory for a channel type.
func RegisterChannelHandler(channelType messagechannel.Type, factory ChannelHandlerFactory) {
	channelHandlerRegistry[channelType] = factory
}

// createChannelHandler creates a ChannelHandler using the registered factory for the given channel type.
func createChannelHandler(ctx context.Context, ch *ent.MessageChannel, routing *MessageRouting) (ChannelHandler, error) {
	factory, ok := channelHandlerRegistry[ch.Type]
	if !ok {
		log.Debug(ctx, "no handler registered for channel type",
			log.Int("channel_id", ch.ID),
			log.String("type", string(ch.Type)))

		return nil, nil
	}

	return factory(ctx, ch, routing)
}
