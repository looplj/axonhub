package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelOverrideTemplateService_CreateTemplate(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
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
		headerOps := []objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "Authorization", Value: "Bearer token"},
		}
		bodyOps := []objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "temperature", Value: "0.7"},
		}
		description := "Test description"

		input := ent.CreateChannelOverrideTemplateInput{
			Name:                     "Test Template",
			Description:              &description,
			HeaderOverrideOperations: headerOps,
			BodyOverrideOperations:   bodyOps,
		}

		template, err := service.CreateTemplate(
			ctx,
			user.ID,
			input,
		)

		require.NoError(t, err)
		require.Equal(t, "Test Template", template.Name)
		require.Equal(t, "Test description", template.Description)
		require.Equal(t, headerOps, template.HeaderOverrideOperations)
		require.Equal(t, bodyOps, template.BodyOverrideOperations)
		require.Equal(t, user.ID, template.UserID)
	})

	t.Run("reject invalid body operations", func(t *testing.T) {
		bodyOps := []objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "", Value: "value"}, // empty path
		}

		input := ent.CreateChannelOverrideTemplateInput{
			Name:                     "Invalid BodyOps Template",
			Description:              nil,
			HeaderOverrideOperations: nil,
			BodyOverrideOperations:   bodyOps,
		}

		_, err := service.CreateTemplate(
			ctx,
			user.ID,
			input,
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid body override operations")
	})

	t.Run("reject stream parameter", func(t *testing.T) {
		bodyOps := []objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "stream", Value: "true"},
		}

		input := ent.CreateChannelOverrideTemplateInput{
			Name:                     "Stream Template",
			Description:              nil,
			HeaderOverrideOperations: nil,
			BodyOverrideOperations:   bodyOps,
		}

		_, err := service.CreateTemplate(
			ctx,
			user.ID,
			input,
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "stream")
	})

	t.Run("reject invalid headers", func(t *testing.T) {
		headerOps := []objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "", Value: "value"}, // empty path
		}

		input := ent.CreateChannelOverrideTemplateInput{
			Name:                     "Invalid Headers Template",
			Description:              nil,
			HeaderOverrideOperations: headerOps,
			BodyOverrideOperations:   nil,
		}

		_, err := service.CreateTemplate(
			ctx,
			user.ID,
			input,
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid header override operations")
	})
}

func TestChannelOverrideTemplateService_UpdateTemplate(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
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
		SetBodyOverrideOperations([]objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "temperature", Value: "0.7"}}).
		SetHeaderOverrideOperations([]objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "X-API-Key", Value: "key1"}}).
		SaveX(ctx)

	t.Run("update name only", func(t *testing.T) {
		newName := "Updated Name"
		input := ent.UpdateChannelOverrideTemplateInput{
			Name: &newName,
		}
		updated, err := service.UpdateTemplate(ctx, template.ID, input)

		require.NoError(t, err)
		require.Equal(t, newName, updated.Name)
		require.Equal(t, "Original description", updated.Description)
	})

	t.Run("update body operations", func(t *testing.T) {
		newBodyOps := []objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "max_tokens", Value: "1000"}}
		input := ent.UpdateChannelOverrideTemplateInput{
			BodyOverrideOperations: newBodyOps,
		}
		updated, err := service.UpdateTemplate(ctx, template.ID, input)

		require.NoError(t, err)
		require.Equal(t, newBodyOps, updated.BodyOverrideOperations)
	})

	t.Run("update header operations", func(t *testing.T) {
		newHeaderOps := []objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "Authorization", Value: "Bearer token"}}
		input := ent.UpdateChannelOverrideTemplateInput{
			HeaderOverrideOperations: newHeaderOps,
		}
		updated, err := service.UpdateTemplate(ctx, template.ID, input)

		require.NoError(t, err)
		require.Equal(t, newHeaderOps, updated.HeaderOverrideOperations)
	})

	t.Run("reject invalid body operations on update", func(t *testing.T) {
		invalidBodyOps := []objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "", Value: "value"}}
		input := ent.UpdateChannelOverrideTemplateInput{
			BodyOverrideOperations: invalidBodyOps,
		}
		_, err := service.UpdateTemplate(ctx, template.ID, input)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid body override operations")
	})
}

func TestChannelOverrideTemplateService_ApplyTemplate(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
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

	// Create template with new operations fields
	template := client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("Test Template").
		SetBodyOverrideOperations([]objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "temperature", Value: "0.9"},
			{Op: objects.OverrideOpSet, Path: "max_tokens", Value: "2000"},
		}).
		SetHeaderOverrideOperations([]objects.OverrideOperation{
			{Op: objects.OverrideOpSet, Path: "X-Custom-Header", Value: "custom-value"},
		}).
		SaveX(ctx)

	t.Run("apply template to channels with new operations merge", func(t *testing.T) {
		// Create channels with existing settings using new operations fields
		ch1 := client.Channel.Create().
			SetName("Channel 1").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{
				BodyOverrideOperations: []objects.OverrideOperation{
					{Op: objects.OverrideOpSet, Path: "temperature", Value: "0.7"},
					{Op: objects.OverrideOpSet, Path: "top_p", Value: "0.9"},
				},
				HeaderOverrideOperations: []objects.OverrideOperation{
					{Op: objects.OverrideOpSet, Path: "Authorization", Value: "Bearer token"},
				},
			}).
			SaveX(ctx)

		ch2 := client.Channel.Create().
			SetName("Channel 2").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{}).
			SaveX(ctx)

		updated, err := service.ApplyTemplate(ctx, template.ID, []int{ch1.ID, ch2.ID})

		require.NoError(t, err)
		require.Len(t, updated, 2)

		// Verify channel 1 merged correctly - template overrides existing values
		require.Len(t, updated[0].Settings.BodyOverrideOperations, 3)
		require.Contains(t, updated[0].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "temperature", Value: "0.9"}) // overridden by template
		require.Contains(t, updated[0].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "top_p", Value: "0.9"})       // preserved from existing
		require.Contains(t, updated[0].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "max_tokens", Value: "2000"}) // from template

		require.Len(t, updated[0].Settings.HeaderOverrideOperations, 2)
		require.Contains(t, updated[0].Settings.HeaderOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "Authorization", Value: "Bearer token"})
		require.Contains(t, updated[0].Settings.HeaderOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "X-Custom-Header", Value: "custom-value"})

		// Verify legacy fields are cleared
		require.Empty(t, updated[0].Settings.OverrideParameters)
		require.Empty(t, updated[0].Settings.OverrideHeaders)

		// Verify channel 2 has template operations
		require.Len(t, updated[1].Settings.BodyOverrideOperations, 2)
		require.Contains(t, updated[1].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "temperature", Value: "0.9"})
		require.Contains(t, updated[1].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "max_tokens", Value: "2000"})

		require.Len(t, updated[1].Settings.HeaderOverrideOperations, 1)
		require.Contains(t, updated[1].Settings.HeaderOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "X-Custom-Header", Value: "custom-value"})
	})

	t.Run("apply template to channels with legacy fields", func(t *testing.T) {
		// Create template with new operations
		templateNew := client.ChannelOverrideTemplate.Create().
			SetUserID(user.ID).
			SetName("Test Template Legacy").
			SetBodyOverrideOperations([]objects.OverrideOperation{
				{Op: objects.OverrideOpSet, Path: "presence_penalty", Value: "0.5"},
			}).
			SetHeaderOverrideOperations([]objects.OverrideOperation{
				{Op: objects.OverrideOpSet, Path: "X-New-Header", Value: "new-value"},
			}).
			SaveX(ctx)

		// Create channel with legacy fields
		ch := client.Channel.Create().
			SetName("Legacy Channel").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{
				OverrideParameters: `{"frequency_penalty": 0.3}`,
				OverrideHeaders:    []objects.HeaderEntry{{Key: "X-Legacy", Value: "legacy-value"}},
			}).
			SaveX(ctx)

		updated, err := service.ApplyTemplate(ctx, templateNew.ID, []int{ch.ID})

		require.NoError(t, err)
		require.Len(t, updated, 1)

		// Verify legacy fields are converted and merged, then cleared
		require.Len(t, updated[0].Settings.BodyOverrideOperations, 2)
		require.Contains(t, updated[0].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "frequency_penalty", Value: "0.3"})
		require.Contains(t, updated[0].Settings.BodyOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "presence_penalty", Value: "0.5"})
		require.Empty(t, updated[0].Settings.OverrideParameters)

		require.Len(t, updated[0].Settings.HeaderOverrideOperations, 2)
		require.Contains(t, updated[0].Settings.HeaderOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "X-Legacy", Value: "legacy-value"})
		require.Contains(t, updated[0].Settings.HeaderOverrideOperations, objects.OverrideOperation{Op: objects.OverrideOpSet, Path: "X-New-Header", Value: "new-value"})
		require.Empty(t, updated[0].Settings.OverrideHeaders)
	})

	t.Run("reject non-existent channel", func(t *testing.T) {
		_, err := service.ApplyTemplate(ctx, template.ID, []int{999999})

		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("rollback on partial failure", func(t *testing.T) {
		// Create one valid channel
		ch := client.Channel.Create().
			SetName("Valid Channel").
			SetType(channel.TypeOpenai).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SaveX(ctx)

		// Try to apply to valid and non-existent channel
		_, err := service.ApplyTemplate(ctx, template.ID, []int{ch.ID, 999999})

		// Should fail and rollback
		require.Error(t, err)

		// Verify original channel wasn't modified
		reloaded := client.Channel.GetX(ctx, ch.ID)
		// Channel will have empty settings, not nil
		if reloaded.Settings != nil {
			require.Empty(t, reloaded.Settings.BodyOverrideOperations)
			require.Empty(t, reloaded.Settings.HeaderOverrideOperations)
		}
	})
}

func TestChannelOverrideTemplateService_DeleteTemplate(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
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
		SaveX(ctx)

	err := service.DeleteTemplate(ctx, template.ID)
	require.NoError(t, err)

	// Verify soft delete
	_, err = client.ChannelOverrideTemplate.Get(ctx, template.ID)
	require.Error(t, err)
}

func TestChannelOverrideTemplateService_QueryTemplates(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
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
		SaveX(ctx)

	client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("OpenAI Template 2").
		SaveX(ctx)

	client.ChannelOverrideTemplate.Create().
		SetUserID(user.ID).
		SetName("Anthropic Template").
		SaveX(ctx)

	t.Run("query all templates", func(t *testing.T) {
		first := 10
		input := QueryChannelOverrideTemplatesInput{
			First: &first,
		}

		conn, err := service.QueryTemplates(ctx, input)
		require.NoError(t, err)
		require.Len(t, conn.Edges, 3)
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
		require.Len(t, conn.Edges, 1)
		require.Contains(t, conn.Edges[0].Node.Name, "Anthropic")
	})
}
