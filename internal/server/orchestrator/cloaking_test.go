package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	entchannel "github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelclientid"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

func TestCloakingProcessor(t *testing.T) {
	ctx := context.Background()

	t.Run("structured order for claudecode official", func(t *testing.T) {
		autoMode := "auto"
		disableSensitive := "disable"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:               &autoMode,
			SensitiveWordsMode: &disableSensitive,
		}

		testCtx, inbound := createTestInboundWithConfig(ctx, t, "claudecode", nil, globalConfig, true)
		middleware := applyStructuredCloaking(inbound)

		userText := "hello"
		request := &llm.Request{
			Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &userText}}},
		}

		processed, err := middleware.OnInboundLlmRequest(testCtx, request)
		require.NoError(t, err)
		require.NotNil(t, processed)
		require.GreaterOrEqual(t, len(processed.Messages), 3)
		require.Equal(t, "system", processed.Messages[0].Role)
		require.NotNil(t, processed.Messages[0].Content.Content)
		require.Contains(t, *processed.Messages[0].Content.Content, "x-anthropic-billing-header")
		require.Equal(t, "system", processed.Messages[1].Role)
		require.NotNil(t, processed.Messages[1].Content.Content)
		require.Equal(t, "You are Claude Code, Anthropic's official CLI for Claude.", *processed.Messages[1].Content.Content)
		require.NotNil(t, processed.Metadata)
		require.NotEmpty(t, processed.Metadata["user_id"])
		require.NotNil(t, processed.TransformerMetadata)
		require.Equal(t, true, processed.TransformerMetadata[llm.TransformerMetadataKeyCloakingCacheControlAutoInject])
	})

	t.Run("mode gate blocks sub switch bypass", func(t *testing.T) {
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:      nil,
			BodyCloak: testBoolPtr(true),
		}

		testCtx, inbound := createTestInboundWithConfig(ctx, t, "claudecode", nil, globalConfig, true)
		middleware := applyStructuredCloaking(inbound)

		text := "hello"
		request := &llm.Request{
			Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &text}}},
		}

		processed, err := middleware.OnInboundLlmRequest(testCtx, request)
		require.NoError(t, err)
		require.NotNil(t, processed)
		require.Equal(t, 1, len(processed.Messages))
		require.Nil(t, processed.Metadata)
	})

	t.Run("dedupe preserves single billing and system prompt", func(t *testing.T) {
		autoMode := "auto"
		disableSensitive := "disable"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:               &autoMode,
			SensitiveWordsMode: &disableSensitive,
		}

		testCtx, inbound := createTestInboundWithConfig(ctx, t, "claudecode", nil, globalConfig, true)
		middleware := applyStructuredCloaking(inbound)

		billing := "x-anthropic-billing-header: cc_version=2.1.78.abc; cc_entrypoint=cli;"
		system := "You are Claude Code, Anthropic's official CLI for Claude."
		userText := "hello"
		request := &llm.Request{
			Messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: &billing}},
				{Role: "system", Content: llm.MessageContent{Content: &system}},
				{Role: "user", Content: llm.MessageContent{Content: &userText}},
			},
		}

		processed, err := middleware.OnInboundLlmRequest(testCtx, request)
		require.NoError(t, err)
		require.NotNil(t, processed)

		billingCount := 0
		systemCount := 0
		for _, msg := range processed.Messages {
			if msg.Role != "system" || msg.Content.Content == nil {
				continue
			}
			if *msg.Content.Content == system {
				systemCount++
			}
			if len(*msg.Content.Content) >= len("x-anthropic-billing-header") && (*msg.Content.Content)[:len("x-anthropic-billing-header")] == "x-anthropic-billing-header" {
				billingCount++
			}
		}

		require.Equal(t, 1, billingCount)
		require.Equal(t, 1, systemCount)
	})

	t.Run("runtime uses persisted client id for api key principal", func(t *testing.T) {
		autoMode := "auto"
		disableSensitive := "disable"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:               &autoMode,
			SensitiveWordsMode: &disableSensitive,
		}

		testCtx, inbound := createTestInboundWithConfig(ctx, t, "claudecode", nil, globalConfig, false)
		middleware := applyStructuredCloaking(inbound)

		userText := "hello"
		request := &llm.Request{Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &userText}}}}

		processedFirst, err := middleware.OnInboundLlmRequest(testCtx, request)
		require.NoError(t, err)
		require.NotNil(t, processedFirst)
		require.NotNil(t, processedFirst.Metadata)

		uidFirst := claudecode.ParseUserID(processedFirst.Metadata["user_id"])
		require.NotNil(t, uidFirst)

		processedSecond, err := middleware.OnInboundLlmRequest(testCtx, request)
		require.NoError(t, err)
		require.NotNil(t, processedSecond)
		require.NotNil(t, processedSecond.Metadata)

		uidSecond := claudecode.ParseUserID(processedSecond.Metadata["user_id"])
		require.NotNil(t, uidSecond)
		require.Equal(t, uidFirst.ClientIDHex, uidSecond.ClientIDHex)

		client := ent.FromContext(testCtx)
		require.NotNil(t, client)
		records, err := client.ChannelClientID.Query().All(testCtx)
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.Equal(t, "api_key", records[0].PrincipalKind)
		require.Equal(t, biz.ComputePrincipalHash("api_key", "runtime-api-key"), records[0].PrincipalHash)
		require.Equal(t, uidFirst.ClientIDHex, records[0].ClientIDHex)
	})

	t.Run("runtime uses oauth principal hash marker", func(t *testing.T) {
		autoMode := "auto"
		disableSensitive := "disable"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:               &autoMode,
			SensitiveWordsMode: &disableSensitive,
		}

		testCtx, inbound := createTestInboundWithConfig(ctx, t, "claudecode", nil, globalConfig, true)
		middleware := applyStructuredCloaking(inbound)

		userText := "hello"
		request := &llm.Request{Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &userText}}}}

		processed, err := middleware.OnInboundLlmRequest(testCtx, request)
		require.NoError(t, err)
		require.NotNil(t, processed)
		require.NotNil(t, processed.Metadata)
		require.NotNil(t, claudecode.ParseUserID(processed.Metadata["user_id"]))

		client := ent.FromContext(testCtx)
		require.NotNil(t, client)
		record, err := client.ChannelClientID.Query().Where(channelclientid.ChannelIDEQ(1)).Only(testCtx)
		require.NoError(t, err)
		require.Equal(t, "oauth", record.PrincipalKind)
		require.Equal(t, "__oauth__", record.PrincipalHash)
	})
}

func createTestInboundWithConfig(
	parentCtx context.Context,
	t *testing.T,
	channelType string,
	channelSettings *objects.ChannelSettings,
	globalConfig *biz.GlobalCloakingConfig,
	isOAuth bool,
) (context.Context, *PersistentInboundTransformer) {
	testCtx, outbound := createTestOutboundWithConfig(parentCtx, t, channelSettings, globalConfig)
	if outbound.state != nil && outbound.state.CurrentCandidate != nil && outbound.state.CurrentCandidate.Channel != nil {
		outbound.state.CurrentCandidate.Channel.Type = entchannel.Type(channelType)
		if isOAuth {
			outbound.state.CurrentCandidate.Channel.Credentials = objects.ChannelCredentials{
				OAuth: &objects.OAuthCredentials{AccessToken: "oauth-token"},
			}
		} else {
			outbound.state.APIKey = &ent.APIKey{Key: "runtime-api-key"}
		}
	}
	return testCtx, &PersistentInboundTransformer{state: outbound.state}
}
