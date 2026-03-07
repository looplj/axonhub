package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/samber/lo"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/messagechannel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

func init() {
	RegisterChannelHandler(messagechannel.TypeFeishu, newFeishuHandler)
}

type feishuHandler struct {
	channel    *ent.MessageChannel
	feishu     *objects.FeishuSettings
	wsClient   *larkws.Client
	larkClient *lark.Client
	routing    *MessageRouting
	cancel     context.CancelFunc
	botOpenID  atomic.Value
}

func newFeishuHandler(ctx context.Context, ch *ent.MessageChannel, routing *MessageRouting) (ChannelHandler, error) {
	settings := ch.Settings
	if settings.Feishu == nil || settings.Feishu.AppID == "" || settings.Feishu.AppSecret == "" {
		return nil, fmt.Errorf("feishu channel missing credentials")
	}

	feishu := settings.Feishu
	larkClient := lark.NewClient(feishu.AppID, feishu.AppSecret)

	h := &feishuHandler{
		channel:    ch,
		feishu:     feishu,
		larkClient: larkClient,
		routing:    routing,
	}

	dispatcher := larkdispatcher.NewEventDispatcher(feishu.VerificationToken, feishu.EncryptKey).
		OnP2MessageReceiveV1(h.handleMessage)

	wsClient := larkws.NewClient(
		feishu.AppID,
		feishu.AppSecret,
		larkws.WithEventHandler(dispatcher),
	)
	h.wsClient = wsClient

	return h, nil
}

func (h *feishuHandler) Start(ctx context.Context) error {
	if err := h.fetchBotOpenID(ctx); err != nil {
		log.Warn(ctx, "failed to fetch bot open_id, @mention detection may not work",
			log.Int("channel_id", h.channel.ID),
			log.Cause(err))
	}

	runCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	go func() {
		if err := h.wsClient.Start(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error(ctx, "feishu websocket error",
				log.Int("channel_id", h.channel.ID),
				log.Cause(err))
		}
	}()

	return nil
}

func (h *feishuHandler) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
}

func (h *feishuHandler) SendMessage(ctx context.Context, msg OutboundMessage) error {
	cardContent, err := buildMarkdownCard(msg.Content)
	if err != nil {
		return fmt.Errorf("build card: %w", err)
	}

	if msg.ReplyToMessageID != "" {
		err = h.replyMessage(ctx, msg.ReplyToMessageID, cardContent)
		if err == nil {
			return nil
		}

		log.Warn(ctx, "reply failed, falling back to direct send",
			log.String("reply_to", msg.ReplyToMessageID),
			log.Cause(err))
	}

	return h.sendDirectMessage(ctx, msg.ChatID, cardContent)
}

func (h *feishuHandler) replyMessage(ctx context.Context, replyToMessageID, cardContent string) error {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(replyToMessageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Content(cardContent).
			Build()).
		Build()

	resp, err := h.larkClient.Im.V1.Message.Reply(ctx, req)
	if err != nil {
		return fmt.Errorf("reply message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu reply api error (code=%d msg=%s)", resp.Code, resp.Msg)
	}

	return nil
}

func (h *feishuHandler) sendDirectMessage(ctx context.Context, chatID, cardContent string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).
			Content(cardContent).
			Build()).
		Build()

	resp, err := h.larkClient.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu api error (code=%d msg=%s)", resp.Code, resp.Msg)
	}

	return nil
}

// handleMessage converts a Feishu event into an InboundMessage and delegates to MessageRouting.
func (h *feishuHandler) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		log.Debug(ctx, "received nil event or message")
		return nil
	}

	msg, ok := h.toInboundMessage(ctx, event)
	if !ok {
		return nil
	}

	return h.routing.HandleInbound(ctx, msg)
}

func (h *feishuHandler) toInboundMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) (InboundMessage, bool) {
	message := event.Event.Message
	sender := event.Event.Sender

	chatID := lo.FromPtr(message.ChatId)
	if chatID == "" {
		log.Debug(ctx, "message has no chat_id")
		return InboundMessage{}, false
	}

	senderID := extractFeishuSenderID(sender)
	if senderID == "" {
		senderID = "unknown"
	}

	messageType := lo.FromPtr(message.MessageType)
	messageID := lo.FromPtr(message.MessageId)
	rawContent := lo.FromPtr(message.Content)
	chatType := lo.FromPtr(message.ChatType)

	log.Debug(ctx, "received feishu message",
		log.Int("channel_id", h.channel.ID),
		log.String("chat_id", chatID),
		log.String("sender_id", senderID),
		log.String("message_type", messageType),
		log.String("message_id", messageID))

	content := extractFeishuContent(messageType, rawContent)

	var isBotMentioned bool

	if chatType != "p2p" && len(message.Mentions) > 0 {
		content = stripMentionPlaceholders(content, message.Mentions)
		isBotMentioned = h.isBotMentioned(message)
	}

	return InboundMessage{
		SenderID:  senderID,
		ChatID:    chatID,
		ChatType:  lo.Ternary(chatType == "p2p", objects.MessageChatTypeDM, objects.MessageChatTypeGroup),
		MessageID: messageID,
		Content:   content,
		Mentioned: isBotMentioned,
	}, true
}

func extractFeishuSenderID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}

	if sender.SenderId.UserId != nil && *sender.SenderId.UserId != "" {
		return *sender.SenderId.UserId
	}

	if sender.SenderId.OpenId != nil && *sender.SenderId.OpenId != "" {
		return *sender.SenderId.OpenId
	}

	if sender.SenderId.UnionId != nil && *sender.SenderId.UnionId != "" {
		return *sender.SenderId.UnionId
	}

	return ""
}

func extractFeishuContent(messageType, rawContent string) string {
	if rawContent == "" {
		return ""
	}

	switch messageType {
	case larkim.MsgTypeText:
		var textPayload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(rawContent), &textPayload); err == nil {
			return textPayload.Text
		}

		return rawContent

	case larkim.MsgTypePost:
		return rawContent

	default:
		return rawContent
	}
}

func (h *feishuHandler) isBotMentioned(message *larkim.EventMessage) bool {
	if message.Mentions == nil {
		return false
	}

	knownID, _ := h.botOpenID.Load().(string)
	if knownID == "" {
		log.Debug(context.Background(), "bot open_id unknown, cannot detect @mention")
		return false
	}

	for _, m := range message.Mentions {
		if m.Id == nil {
			continue
		}

		if m.Id.OpenId != nil && *m.Id.OpenId == knownID {
			return true
		}
	}

	return false
}

func (h *feishuHandler) fetchBotOpenID(ctx context.Context) error {
	resp, err := h.larkClient.Do(ctx, &larkcore.ApiReq{
		HttpMethod:                http.MethodGet,
		ApiPath:                   "/open-apis/bot/v3/info",
		SupportedAccessTokenTypes: []larkcore.AccessTokenType{larkcore.AccessTokenTypeTenant},
	})
	if err != nil {
		return fmt.Errorf("bot info request: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Bot  struct {
			OpenID string `json:"open_id"`
		} `json:"bot"`
	}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return fmt.Errorf("bot info parse: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("bot info api error (code=%d)", result.Code)
	}

	if result.Bot.OpenID == "" {
		return fmt.Errorf("bot info: empty open_id")
	}

	h.botOpenID.Store(result.Bot.OpenID)
	log.Debug(ctx, "fetched bot open_id from API",
		log.Int("channel_id", h.channel.ID),
		log.String("open_id", result.Bot.OpenID))

	return nil
}

func stripMentionPlaceholders(content string, mentions []*larkim.MentionEvent) string {
	if len(mentions) == 0 {
		return content
	}

	for _, m := range mentions {
		if m.Key != nil && *m.Key != "" {
			content = strings.ReplaceAll(content, *m.Key, "")
		}
	}

	return strings.TrimSpace(content)
}

func buildMarkdownCard(content string) (string, error) {
	card := map[string]any{
		"schema": "2.0",
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":     "markdown",
					"content": content,
				},
			},
		},
	}

	data, err := json.Marshal(card)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
