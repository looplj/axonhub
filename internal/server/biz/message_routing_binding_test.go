package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/messagechannel"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

func TestMessageRouting_HandleInbound_GroupMentionRequirement(t *testing.T) {
	tests := []struct {
		name                string
		allowWithoutMention bool
		mentioned           bool
		wantMessages        int
	}{
		{
			name:         "default binding still requires mention",
			mentioned:    false,
			wantMessages: 0,
		},
		{
			name:                "binding can opt out of mention requirement",
			allowWithoutMention: true,
			mentioned:           false,
			wantMessages:        1,
		},
		{
			name:         "mentioned group message still routes by default",
			mentioned:    true,
			wantMessages: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
			defer client.Close()

			ctx := authz.WithTestBypass(context.Background())
			ctx = ent.NewContext(ctx, client)

			channel := createTestMessageRoutingFixture(t, ctx, client, tt.allowWithoutMention)
			routing := &MessageRouting{
				db:      client,
				channel: channel,
			}

			err := routing.HandleInbound(ctx, InboundMessage{
				SenderID:  "user_open_id",
				ChatID:    "oc_group_chat",
				ChatType:  objects.MessageChatTypeGroup,
				MessageID: uuid.NewString(),
				Content:   "hello from group",
				Mentioned: tt.mentioned,
			})
			require.NoError(t, err)

			count, err := client.AgentMessage.Query().Count(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.wantMessages, count)
		})
	}
}

func createTestMessageRoutingFixture(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	allowWithoutMention bool,
) *ent.MessageChannel {
	t.Helper()

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	u, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	p, err := client.Project.Create().
		SetName(uuid.NewString()).
		SetDescription("test project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	sysPrompt, err := client.Prompt.Create().
		SetProjectID(p.ID).
		SetType(prompt.TypeAgent).
		SetName("agent prompt").
		SetRole("system").
		SetContent("test").
		SetStatus(prompt.StatusEnabled).
		SetSettings(objects.PromptSettings{Action: objects.PromptAction{Type: objects.PromptActionTypeNoop}}).
		Save(ctx)
	require.NoError(t, err)

	a, err := client.Agent.Create().
		SetProjectID(p.ID).
		SetCreatedByUserID(u.ID).
		SetPromptID(sysPrompt.ID).
		SetName("agent").
		SetStatus(agent.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	key, err := GenerateAPIKey()
	require.NoError(t, err)

	apiKey, err := client.APIKey.Create().
		SetUserID(u.ID).
		SetProjectID(p.ID).
		SetKey(key).
		SetName("agent api key").
		SetType(apikey.TypeAgent).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.AgentInstance.Create().
		SetProjectID(p.ID).
		SetAgentID(a.ID).
		SetAPIKeyID(apiKey.ID).
		SetName("inst").
		SetStatus(agentinstance.StatusRunning).
		SetLastHeartbeatAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	ch, err := client.MessageChannel.Create().
		SetProjectID(p.ID).
		SetName("channel").
		SetDescription("test channel").
		SetType(messagechannel.TypeFeishu).
		SetStatus(messagechannel.StatusEnabled).
		SetSettings(objects.MessageChannelSettings{}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.MessageChannelAgentInstance.Create().
		SetMessageChannelID(ch.ID).
		SetAgentInstanceID(inst.ID).
		SetEnabled(true).
		SetConfig(objects.MessageChannelAgentInstanceBinding{
			ChatType:            objects.MessageChatTypeGroup,
			ChatID:              "oc_group_chat",
			AllowWithoutMention: allowWithoutMention,
		}).
		Save(ctx)
	require.NoError(t, err)

	return ch
}
