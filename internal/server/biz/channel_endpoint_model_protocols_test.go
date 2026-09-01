//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestValidateModelProtocols(t *testing.T) {
	// TypeXai defaults to openai/chat_completions + openai/responses.
	defaults := DefaultEndpointsForChannelType(channel.TypeXai)
	require.NotEmpty(t, defaults)

	userEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
	}

	t.Run("nil settings or empty list passes", func(t *testing.T) {
		require.NoError(t, ValidateModelProtocols(nil, channel.TypeXai, userEndpoints))
		require.NoError(t, ValidateModelProtocols(&objects.ChannelSettings{}, channel.TypeXai, userEndpoints))
	})

	t.Run("valid entry referencing default and user endpoints", func(t *testing.T) {
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "grok-4", APIFormats: []string{
				llm.APIFormatOpenAIResponse.String(),
				llm.APIFormatAnthropicMessage.String(),
			}},
		}}

		require.NoError(t, ValidateModelProtocols(settings, channel.TypeXai, userEndpoints))
	})

	t.Run("empty model rejected", func(t *testing.T) {
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{APIFormats: []string{llm.APIFormatOpenAIResponse.String()}},
		}}

		require.ErrorContains(t, ValidateModelProtocols(settings, channel.TypeXai, nil), "model is required")
	})

	t.Run("duplicate model rejected", func(t *testing.T) {
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "grok-4", APIFormats: []string{llm.APIFormatOpenAIResponse.String()}},
			{Model: "grok-4", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
		}}

		require.ErrorContains(t, ValidateModelProtocols(settings, channel.TypeXai, nil), "duplicate model")
	})

	t.Run("empty api formats rejected", func(t *testing.T) {
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "grok-4"},
		}}

		require.ErrorContains(t, ValidateModelProtocols(settings, channel.TypeXai, nil), "at least one api_format")
	})

	t.Run("api format not configured on channel rejected", func(t *testing.T) {
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "grok-4", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}},
		}}

		// No user endpoints: anthropic/messages is not a xai default.
		require.ErrorContains(t, ValidateModelProtocols(settings, channel.TypeXai, nil), "not configured on this channel")
	})

	t.Run("disabled entry may retain a removed endpoint format", func(t *testing.T) {
		disabled := false
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "grok-4", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}, Enabled: &disabled},
		}}
		require.NoError(t, ValidateModelProtocols(settings, channel.TypeXai, nil))
	})
}

func TestChannel_ForcedAPIFormats(t *testing.T) {
	disabled := false
	ch := &Channel{Channel: &ent.Channel{
		Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "grok-4", APIFormats: []string{
				llm.APIFormatOpenAIResponse.String(),
				llm.APIFormatAnthropicMessage.String(),
			}},
			{Model: "grok-3", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
			{Model: "disabled-model", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}, Enabled: &disabled},
		}},
	}}

	require.Equal(t, []string{
		llm.APIFormatOpenAIResponse.String(),
		llm.APIFormatAnthropicMessage.String(),
	}, ch.ForcedAPIFormats("grok-4"))
	require.Equal(t, []string{llm.APIFormatOpenAIChatCompletion.String()}, ch.ForcedAPIFormats("grok-3"))
	require.Nil(t, ch.ForcedAPIFormats("disabled-model"))
	require.Nil(t, ch.ForcedAPIFormats("other-model"))
	require.Nil(t, ch.ForcedAPIFormats(""))
	require.Nil(t, (&Channel{}).ForcedAPIFormats("grok-4"))
}

func TestRemoveRemovedModelProtocolOverrides(t *testing.T) {
	settings := &objects.ChannelSettings{
		ExtraModelPrefix: "provider",
		ModelMappings:    []objects.ModelMapping{{From: "alias", To: "gpt-4"}},
		ModelProtocols: []objects.ModelProtocol{
			{Model: "gpt-4", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
			{Model: "alias", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
			{Model: "removed", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
		},
	}

	require.True(t, RemoveRemovedModelProtocolOverrides(settings, []string{"gpt-4"}))
	require.Equal(t, []string{"gpt-4", "alias"}, lo.Map(settings.ModelProtocols, func(protocol objects.ModelProtocol, _ int) string {
		return protocol.Model
	}))
	require.False(t, RemoveRemovedModelProtocolOverrides(settings, []string{"gpt-4"}))
}

func TestCreateChannel_PreservesModelProtocolsWithEmptySupportedModels(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_model_protocols_create?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)
	settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
		{Model: "gpt-4", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
	}}

	created, err := svc.createChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "openai-empty-model-list",
		BaseURL:          lo.ToPtr("https://api.example.com"),
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{},
		DefaultTestModel: "gpt-4",
		Settings:         settings,
	})
	require.NoError(t, err)
	require.Equal(t, settings.ModelProtocols, created.Settings.ModelProtocols)
}

func TestChannelService_DuplicateChannelInheritsCustomEndpointsForModelProtocols(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_model_protocols_duplicate?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)
	customEndpoints := []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}}
	modelProtocols := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
		{Model: "minimax-m3", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}},
	}}

	source, err := client.Channel.Create().
		SetName("minimax-anthropic-source").
		SetType(channel.TypeMinimaxAnthropic).
		SetBaseURL("https://api.minimaxi.com/anthropic").
		SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).
		SetSupportedModels([]string{"minimax-m3"}).
		SetDefaultTestModel("minimax-m3").
		SetEndpoints(customEndpoints).
		SetSettings(modelProtocols).
		Save(ctx)
	require.NoError(t, err)

	duplicated, err := svc.DuplicateChannel(ctx, source.ID, ent.CreateChannelInput{
		Type:             channel.TypeMinimaxAnthropic,
		BaseURL:          lo.ToPtr("https://api.minimaxi.com/anthropic"),
		Name:             "minimax-anthropic-copy",
		Credentials:      objects.ChannelCredentials{APIKey: "copy-key"},
		SupportedModels:  []string{"minimax-m3"},
		DefaultTestModel: "minimax-m3",
		Settings:         modelProtocols,
	})
	require.NoError(t, err)
	require.Equal(t, customEndpoints, duplicated.Endpoints)
	require.Equal(t, modelProtocols.ModelProtocols, duplicated.Settings.ModelProtocols)
}

// TestUpdateChannel_ModelProtocolsValidatedWithoutResentSettings covers updates that
// change the endpoint surface (endpoints or channel type) without resending settings:
// overrides already stored in settings must still reference formats that survive the
// change.
func TestUpdateChannel_ModelProtocolsValidatedWithoutResentSettings(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_model_protocols_update?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)

	// zai defaults only to openai/chat_completions; anthropic/messages comes from a
	// custom endpoint and the stored override references it.
	created, err := client.Channel.Create().
		SetName("zai-multi").
		SetType(channel.TypeZai).
		SetBaseURL("https://api.z.ai").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"glm-5.3"}).
		SetDefaultTestModel("glm-5.3").
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatAnthropicMessage.String()}}).
		SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "glm-5.3", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}},
		}}).
		Save(ctx)
	require.NoError(t, err)

	t.Run("removing a supported model without resending settings removes its override", func(t *testing.T) {
		enabled := true
		modelChannel, err := client.Channel.Create().
			SetName("zai-model-sync").
			SetType(channel.TypeZai).
			SetBaseURL("https://api.z.ai").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"glm-5.3"}).
			SetDefaultTestModel("glm-5.3").
			SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
				{Model: "glm-5.3", APIFormats: []string{llm.APIFormatOpenAIChatCompletion.String()}, Enabled: &enabled},
			}}).
			Save(ctx)
		require.NoError(t, err)

		updated, err := svc.UpdateChannel(ctx, modelChannel.ID, &ent.UpdateChannelInput{SupportedModels: []string{}})
		require.NoError(t, err)
		require.Empty(t, updated.Settings.ModelProtocols)
	})

	t.Run("removing the referenced endpoint without resending settings is rejected", func(t *testing.T) {
		_, err := svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{
			Endpoints: []objects.ChannelEndpoint{},
		})
		require.ErrorContains(t, err, "not configured on this channel")
	})

	t.Run("switching to a type without the referenced format is rejected", func(t *testing.T) {
		// xai defaults to chat_completions + responses; the override references the
		// responses default. zai defaults to chat_completions only, so the type
		// change invalidates the stored override.
		xaiChannel, err := client.Channel.Create().
			SetName("xai-multi").
			SetType(channel.TypeXai).
			SetBaseURL("https://api.x.ai").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"grok-4"}).
			SetDefaultTestModel("grok-4").
			SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
				{Model: "grok-4", APIFormats: []string{llm.APIFormatOpenAIResponse.String()}},
			}}).
			Save(ctx)
		require.NoError(t, err)

		_, err = svc.UpdateChannel(ctx, xaiChannel.ID, &ent.UpdateChannelInput{
			Type: lo.ToPtr(channel.TypeZai),
		})
		require.ErrorContains(t, err, "not configured on this channel")
	})

	t.Run("atomic endpoints and settings update passes", func(t *testing.T) {
		// The frontend commits endpoint deletion together with the override cleanup
		// in a single update: the new overrides are validated against the new
		// endpoints.
		updated, err := svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{
			Endpoints: []objects.ChannelEndpoint{},
			Settings:  &objects.ChannelSettings{},
		})
		require.NoError(t, err)
		require.Empty(t, updated.Settings.ModelProtocols)
		require.Empty(t, updated.Endpoints)
	})

	t.Run("removing a supported model removes its protocol override", func(t *testing.T) {
		enabled := true
		updated, err := svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{
			SupportedModels: []string{},
			Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
				{Model: "glm-5.3", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}, Enabled: &enabled},
			}},
		})
		require.NoError(t, err)
		require.Empty(t, updated.Settings.ModelProtocols)
	})
}
