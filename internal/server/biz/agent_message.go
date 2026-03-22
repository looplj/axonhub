package biz

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agentmessage"
	"github.com/looplj/axonhub/internal/objects"
)

type CreateAgentMessageParams struct {
	ProjectID         int
	AgentID           int
	AgentInstanceID   int
	Direction         agentmessage.Direction
	SenderType        agentmessage.SenderType
	SenderID          *int
	Type              agentmessage.Type
	CorrelationID     string
	Content           objects.JSONRawMessage
	Status            agentmessage.Status
	ExternalMessageID *string
	ReplyToMessageID  *int
}

func CreateAgentMessage(ctx context.Context, client *ent.Client, input CreateAgentMessageParams) (*ent.AgentMessage, error) {
	status := input.Status
	if status == "" {
		status = agentmessage.StatusPending
	}

	var msg *ent.AgentMessage

	for attempt := range 3 {
		nextSeq, err := nextSequence(ctx, client, input.AgentInstanceID)
		if err != nil {
			return nil, fmt.Errorf("get next sequence: %w", err)
		}

		creator := client.AgentMessage.Create().
			SetProjectID(input.ProjectID).
			SetAgentID(input.AgentID).
			SetAgentInstanceID(input.AgentInstanceID).
			SetDirection(input.Direction).
			SetSenderType(input.SenderType).
			SetNillableSenderID(input.SenderID).
			SetType(input.Type).
			SetCorrelationID(input.CorrelationID).
			SetContent(input.Content).
			SetStatus(status).
			SetNillableExternalMessageID(input.ExternalMessageID).
			SetNillableReplyToMessageID(input.ReplyToMessageID).
			SetSequence(nextSeq)

		created, err := creator.Save(ctx)
		if err == nil {
			msg = created
			break
		}

		if ent.IsConstraintError(err) && attempt < 2 {
			continue
		}

		return nil, fmt.Errorf("create agent message: %w", err)
	}

	if msg == nil {
		return nil, fmt.Errorf("create agent message: max retries exceeded")
	}

	return msg, nil
}

func nextSequence(ctx context.Context, client *ent.Client, agentInstanceID int) (int64, error) {
	last, err := client.AgentMessage.Query().
		Where(agentmessage.AgentInstanceIDEQ(agentInstanceID)).
		Order(ent.Desc(agentmessage.FieldSequence)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 1, nil
		}

		return 0, fmt.Errorf("query last sequence: %w", err)
	}

	return last.Sequence + 1, nil
}

type MessageContentType string

const (
	MessageContentTypeText            MessageContentType = "text"
	MessageContentTypePairingSuccess  MessageContentType = "pairing_success"
	MessageContentTypeApprovalRequest MessageContentType = "approval_request"
	MessageContentTypeApprovalResult  MessageContentType = "approval_result"
)

type BaseMessageContent struct {
	Type MessageContentType `json:"type"`
	Text string             `json:"text,omitempty"`
}

type TextMessageContent struct {
	Type     MessageContentType `json:"type"`
	Text     string             `json:"text"`
	ChatID   string             `json:"chat_id,omitempty"`
	ChatType string             `json:"chat_type,omitempty"`
}

type ChatMessageContent struct {
	Type      MessageContentType `json:"type"`
	Text      string             `json:"text"`
	ChatID    string             `json:"chat_id"`
	ChatType  string             `json:"chat_type"`
	MessageID string             `json:"message_id"`
}

type PairingSuccessContent struct {
	Type      MessageContentType `json:"type"`
	Text      string             `json:"text"`
	ChatID    string             `json:"chat_id"`
	ChatType  string             `json:"chat_type"`
	MessageID string             `json:"message_id"`
	ChannelID int                `json:"channel_id"`
}

type ApprovalRequestContent struct {
	Type        MessageContentType `json:"type"`
	RequestID   string             `json:"request_id"`
	Text        string             `json:"text,omitempty"`
	Resources   []json.RawMessage  `json:"resources,omitempty"`
	Scope       string             `json:"scope,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	Granted     bool               `json:"granted,omitempty"`
	Instruction string             `json:"instruction,omitempty"`
}

type ApprovalResultContent struct {
	Type      MessageContentType `json:"type"`
	RequestID string             `json:"request_id"`
	Granted   bool               `json:"granted"`
	Scope     string             `json:"scope"`
	Reason    string             `json:"reason,omitempty"`
	Resources []json.RawMessage  `json:"resources,omitempty"`
}

func MarshalMessageContent(content any) (objects.JSONRawMessage, error) {
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	return objects.JSONRawMessage(raw), nil
}

func MarshalTextContent(text string) objects.JSONRawMessage {
	content := BaseMessageContent{
		Type: MessageContentTypeText,
		Text: text,
	}
	raw, _ := json.Marshal(content)

	return objects.JSONRawMessage(raw)
}

func MarshalChatContent(text, chatID, chatType, messageID string) objects.JSONRawMessage {
	content := ChatMessageContent{
		Type:      MessageContentTypeText,
		Text:      text,
		ChatID:    chatID,
		ChatType:  chatType,
		MessageID: messageID,
	}
	raw, _ := json.Marshal(content)

	return objects.JSONRawMessage(raw)
}

func MarshalPairingSuccessContent(chatID, chatType, messageID string, channelID int) objects.JSONRawMessage {
	content := PairingSuccessContent{
		Type:      MessageContentTypePairingSuccess,
		Text:      "Pairing successful. You are now connected to this chat.",
		ChatID:    chatID,
		ChatType:  chatType,
		MessageID: messageID,
		ChannelID: channelID,
	}
	raw, _ := json.Marshal(content)

	return objects.JSONRawMessage(raw)
}

func MarshalApprovalResultContent(requestID string, granted bool, scope, reason string, resources []json.RawMessage) objects.JSONRawMessage {
	content := ApprovalResultContent{
		Type:      MessageContentTypeApprovalResult,
		RequestID: requestID,
		Granted:   granted,
		Scope:     scope,
		Reason:    reason,
		Resources: resources,
	}
	raw, _ := json.Marshal(content)

	return objects.JSONRawMessage(raw)
}

func ExtractTextFromContent(raw objects.JSONRawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var content BaseMessageContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return ""
	}

	return content.Text
}

func ExtractChatIDFromContent(raw objects.JSONRawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var content struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		return ""
	}

	return content.ChatID
}
