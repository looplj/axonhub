package agentapi

import (
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func requireGUIDType(id objects.GUID, typ string) (int, error) {
	if id.Type != typ || id.ID <= 0 {
		return 0, fmt.Errorf("invalid id: expected %s", typ)
	}
	return id.ID, nil
}

func mapMessage(m *biz.AgentMessageView) *AgentMessage {
	var replyToMessageID *objects.GUID
	if m.ReplyToMessageID != nil && *m.ReplyToMessageID > 0 {
		replyToMessageID = &objects.GUID{Type: ent.TypeAgentMessage, ID: *m.ReplyToMessageID}
	}

	return &AgentMessage{
		ID:                objects.GUID{Type: ent.TypeAgentMessage, ID: m.ID},
		AgentID:           objects.GUID{Type: ent.TypeAgent, ID: m.AgentID},
		Direction:         AgentMessageDirection(m.Direction),
		SenderType:        AgentMessageSenderType(m.SenderType),
		Text:              m.Text,
		Content:           m.Content,
		Type:              AgentMessageType(m.Type),
		CorrelationID:     m.CorrelationID,
		Sequence:          int(m.Sequence),
		Status:            AgentMessageStatus(m.Status),
		CreatedAt:         m.CreatedAt,
		ExternalMessageID: m.ExternalMessageID,
		ReplyToMessageID:  replyToMessageID,
	}
}
