package gql

import (
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func mapAgentChatMessage(v *biz.AgentMessageView) *AgentChatMessage {
	if v == nil {
		return nil
	}

	return &AgentChatMessage{
		ID:         objects.GUID{Type: ent.TypeAgentMessage, ID: v.ID},
		AgentID:    objects.GUID{Type: ent.TypeAgent, ID: v.AgentID},
		ThreadID:   v.ThreadID,
		Direction:  v.Direction,
		SenderType: v.SenderType,
		SenderID:   v.SenderID,
		Text:       v.Text,
		Sequence:   int(v.Sequence),
		Status:     v.Status,
		CreatedAt:  v.CreatedAt,
	}
}
