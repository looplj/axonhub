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
		ID:            objects.GUID{Type: ent.TypeAgentMessage, ID: v.ID},
		AgentID:       objects.GUID{Type: ent.TypeAgent, ID: v.AgentID},
		Direction:     v.Direction,
		SenderType:    v.SenderType,
		SenderID:      v.SenderID,
		Kind:          v.Kind,
		CorrelationID: v.CorrelationID,
		Content:       v.Content,
		Text:          v.Text,
		Sequence:      int(v.Sequence),
		Status:        v.Status,
		CreatedAt:     v.CreatedAt,
	}
}

func mapAgentApprovalRequestMessage(v *biz.AgentApprovalRequestView) *AgentApprovalRequestMessage {
	if v == nil {
		return nil
	}

	return &AgentApprovalRequestMessage{
		ID:            objects.GUID{Type: ent.TypeAgentMessage, ID: v.ID},
		AgentID:       objects.GUID{Type: ent.TypeAgent, ID: v.AgentID},
		CorrelationID: v.CorrelationID,
		Content:       v.Content,
		Sequence:      int(v.Sequence),
		CreatedAt:     v.CreatedAt,
	}
}
