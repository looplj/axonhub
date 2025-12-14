package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channeloverridetemplate"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelOverrideTemplateService_CreateTemplate(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := privacy.DecisionContext(context.Background(), privacy.Allow)

	// Create test user
	user := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		SaveX(ctx)

	service := NewChannelOverrideTemplateService(ChannelOverrideTemplateServiceParams{
		Client:         client,
		ChannelService: nil, // nil is fine for these tests
	})

	t.Run("create template successfully", func(t *testing.T) {
		headers := []objects.HeaderEntry{
			{Key: "Authorization", Value: "Bearer token"},
		}
		params := `{"temperature": 0.7}`

		template, err := service.CreateTemplate(
			ctx,
			user.ID,
			"Test Template",
			"Test description",
			string(channel.TypeOpenai),
			params,
			headers,
		)

		require.NoError(t, err)
		assert.Equal(t, "Test Template", template.Name)
		assert.Equal(t, "Test description", template.Description)
		assert.Equal(t, channeloverridetemplate.ChannelTypeOpenai, template.ChannelType)
		assert.Equal(t, params, template.OverrideParameters)
		assert.Equal(t, headers, template.OverrideHeaders)
		assert.Equal(t, user.ID, template.UserID)
	})

	t.Run("reject invalid parameters", func(t *testing.T) {
		_, err := service.CreateTemplate(
			ctx,
			user.ID,
			"Invalid Params Template",
			"",
			string(channel.TypeOpenai),
			`{invalid}`,
			nil,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid override parameters")
	})

	t.Run("reject stream parameter", func(t *testing.T) {
		_, err := service.CreateTemplate(
			ctx,
			user.ID,
			"Stream Template",
			"",
			string(channel.TypeOpenai),
			`{"stream": true}`,
			nil,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "stream")
	})

	t.Run("reject invalid headers", func(t *testing.T) {
		headers := []objects.HeaderEntry{
			{Key: "Authorization", Value: "Bearer token"},
			{Key: "authorization", Value: "Bearer token2"}, // duplicate
		}

		_, err := service.CreateTemplate(
			ctx,
			user.ID,
			"Duplicate Headers Template",
			"",
			string(channel.TypeOpenai),
			"{}",
			headers,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid override headers")
	})
}

func TestChannelOverrideTemplateService_UpdateTemplate(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := privacy.DecisionContext(context.Background(), privacy.Allow)

	user := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		SaveX(ctx)

	service := NewChannelOverrideTemplateService(ChannelOverrideTemplateServiceParams{
		Client:         client,
		ChannelService: nil,
	})

	// Create initial template
	template := client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("Original Name").
		SetDescription("Original description").
		SetChannelType(channeloverridetemplate.ChannelTypeOpenai).
		SetOverrideParameters(`{"temperature": 0.7}`).
		SetOverrideHeaders([]objects.HeaderEntry{{Key: "X-API-Key", Value: "key1"}}).
		SaveX(ctx)

	t.Run("update name only", func(t *testing.T) {
		newName := "Updated Name"
		updated, err := service.UpdateTemplate(ctx, template.ID, &newName, nil, nil, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, newName, updated.Name)
		assert.Equal(t, "Original description", updated.Description)
	})

	t.Run("update parameters", func(t *testing.T) {
		newParams := `{"max_tokens": 1000}`
		updated, err := service.UpdateTemplate(ctx, template.ID, nil, nil, &newParams, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, newParams, updated.OverrideParameters)
	})

	t.Run("update headers", func(t *testing.T) {
		newHeaders := []objects.HeaderEntry{{Key: "Authorization", Value: "Bearer token"}}
		updated, err := service.UpdateTemplate(ctx, template.ID, nil, nil, nil, newHeaders, nil)

		require.NoError(t, err)
		assert.Equal(t, newHeaders, updated.OverrideHeaders)
	})

	t.Run("reject invalid parameters on update", func(t *testing.T) {
		invalidParams := `{invalid}`
		_, err := service.UpdateTemplate(ctx, template.ID, nil, nil, &invalidParams, nil, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid override parameters")
	})
}

func TestChannelOverrideTemplateService_ApplyTemplate(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := privacy.DecisionContext(context.Background(), privacy.Allow)

	user := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		SaveX(ctx)

	service := NewChannelOverrideTemplateService(ChannelOverrideTemplateServiceParams{
		Client:         client,
		ChannelService: nil,
	})

	// Create template
	template := client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("Test Template").
		SetChannelType(channeloverridetemplate.ChannelTypeOpenai).
		SetOverrideParameters(`{"temperature": 0.9, "max_tokens": 2000}`).
		SetOverrideHeaders([]objects.HeaderEntry{
			{Key: "X-Custom-Header", Value: "custom-value"},
		}).
		SaveX(ctx)

	t.Run("apply template to channels with merge", func(t *testing.T) {
		// Create channels with existing settings
		ch1 := client.Channel.Create().
			SetName("Channel 1").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(&objects.ChannelCredentials{APIKey: "key1"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{
				OverrideParameters: `{"temperature": 0.7, "top_p": 0.9}`,
				OverrideHeaders: []objects.HeaderEntry{
					{Key: "Authorization", Value: "Bearer token"},
				},
			}).
			SaveX(ctx)

		ch2 := client.Channel.Create().
			SetName("Channel 2").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(&objects.ChannelCredentials{APIKey: "key2"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{
				OverrideParameters: `{}`,
				OverrideHeaders:    []objects.HeaderEntry{},
			}).
			SaveX(ctx)

		updated, err := service.ApplyTemplate(ctx, template.ID, []int{ch1.ID, ch2.ID})

		require.NoError(t, err)
		assert.Len(t, updated, 2)

		// Verify channel 1 merged correctly
		assert.JSONEq(t, `{"temperature": 0.9, "max_tokens": 2000, "top_p": 0.9}`, updated[0].Settings.OverrideParameters)
		assert.Len(t, updated[0].Settings.OverrideHeaders, 2)
		assert.Contains(t, updated[0].Settings.OverrideHeaders, objects.HeaderEntry{Key: "Authorization", Value: "Bearer token"})
		assert.Contains(t, updated[0].Settings.OverrideHeaders, objects.HeaderEntry{Key: "X-Custom-Header", Value: "custom-value"})

		// Verify channel 2 merged correctly
		assert.JSONEq(t, `{"temperature": 0.9, "max_tokens": 2000}`, updated[1].Settings.OverrideParameters)
		assert.Equal(t, []objects.HeaderEntry{{Key: "X-Custom-Header", Value: "custom-value"}}, updated[1].Settings.OverrideHeaders)
	})

	t.Run("reject mismatched channel type", func(t *testing.T) {
		ch := client.Channel.Create().
			SetName("Anthropic Channel").
			SetType(channel.TypeAnthropic).
			SetBaseURL("https://api.anthropic.com").
			SetCredentials(&objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"claude-3-opus-20240229"}).
			SetDefaultTestModel("claude-3-opus-20240229").
			SaveX(ctx)

		_, err := service.ApplyTemplate(ctx, template.ID, []int{ch.ID})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not match template type")
	})

	t.Run("reject non-existent channel", func(t *testing.T) {
		_, err := service.ApplyTemplate(ctx, template.ID, []int{999999})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("rollback on partial failure", func(t *testing.T) {
		// Create one valid channel
		ch := client.Channel.Create().
			SetName("Valid Channel").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(&objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SaveX(ctx)

		// Try to apply to valid and non-existent channel
		_, err := service.ApplyTemplate(ctx, template.ID, []int{ch.ID, 999999})

		// Should fail and rollback
		assert.Error(t, err)

		// Verify original channel wasn't modified
		reloaded := client.Channel.GetX(ctx, ch.ID)
		// Channel will have empty settings, not nil
		if reloaded.Settings != nil {
			assert.Empty(t, reloaded.Settings.OverrideParameters)
			assert.Empty(t, reloaded.Settings.OverrideHeaders)
		}
	})
}

func TestChannelOverrideTemplateService_DeleteTemplate(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := privacy.DecisionContext(context.Background(), privacy.Allow)

	user := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		SaveX(ctx)

	service := NewChannelOverrideTemplateService(ChannelOverrideTemplateServiceParams{
		Client:         client,
		ChannelService: nil,
	})

	template := client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("Template to Delete").
		SetChannelType(channeloverridetemplate.ChannelTypeOpenai).
		SaveX(ctx)

	err := service.DeleteTemplate(ctx, template.ID)
	require.NoError(t, err)

	// Verify soft delete
	_, err = client.ChannelOverrideTemplate.Get(ctx, template.ID)
	assert.Error(t, err)
}

func TestChannelOverrideTemplateService_QueryTemplates(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := privacy.DecisionContext(context.Background(), privacy.Allow)

	user := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		SaveX(ctx)

	service := NewChannelOverrideTemplateService(ChannelOverrideTemplateServiceParams{
		Client:         client,
		ChannelService: nil,
	})

	// Create test templates
	client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("OpenAI Template 1").
		SetChannelType(channeloverridetemplate.ChannelTypeOpenai).
		SaveX(ctx)

	client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("OpenAI Template 2").
		SetChannelType(channeloverridetemplate.ChannelTypeOpenai).
		SaveX(ctx)

	client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("Anthropic Template").
		SetChannelType(channeloverridetemplate.ChannelTypeAnthropic).
		SaveX(ctx)

	t.Run("query all templates", func(t *testing.T) {
		first := 10
		input := QueryChannelOverrideTemplatesInput{
			First: &first,
		}

		conn, err := service.QueryTemplates(ctx, input)
		require.NoError(t, err)
		assert.Len(t, conn.Edges, 3)
	})

	t.Run("filter by channel type", func(t *testing.T) {
		first := 10
		channelType := channel.TypeOpenai
		input := QueryChannelOverrideTemplatesInput{
			First:       &first,
			ChannelType: &channelType,
		}

		conn, err := service.QueryTemplates(ctx, input)
		require.NoError(t, err)
		assert.Len(t, conn.Edges, 2)
	})

	t.Run("search by name", func(t *testing.T) {
		first := 10
		search := "Anthropic"
		input := QueryChannelOverrideTemplatesInput{
			First:  &first,
			Search: &search,
		}

		conn, err := service.QueryTemplates(ctx, input)
		require.NoError(t, err)
		assert.Len(t, conn.Edges, 1)
		assert.Contains(t, conn.Edges[0].Node.Name, "Anthropic")
	})
}
