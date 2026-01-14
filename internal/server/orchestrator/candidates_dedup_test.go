package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestDefaultSelector_Select_Deduplication(t *testing.T) {
	ctx, client := setupTest(t)

	// Create a channel with multiple RequestModels mapping to the same ActualModel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Deduplication Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetSettings(&objects.ChannelSettings{
			ModelMappings: []objects.ModelMapping{
				{From: "gpt4", To: "gpt-4"},
				{From: "gpt-4-custom", To: "gpt-4"},
			},
		}).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)
	selector := NewDefaultSelector(channelService, modelService, systemService)

	// Create a model with a regex association that matches multiple RequestModels
	// gpt-4 matches "gpt-4"
	// gpt4 matches "gpt4"
	// gpt-4-custom matches "gpt-4-custom"
	// If we use regex "gpt.*", it will match all of them.

	model, err := client.Model.Create().
		SetModelID("my-gpt-4").
		SetName("My GPT-4").
		SetDeveloper("openai").
		SetIcon("openai").
		SetGroup("gpt-4").
		SetModelCard(&objects.ModelCard{}).
		SetStatus("enabled").
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type:     "regex",
					Priority: 1,
					Regex: &objects.RegexAssociation{
						Pattern: "gpt.*",
					},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	req := &llm.Request{
		Model: model.ModelID,
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	require.Len(t, result, 1, "Should only have one candidate for the same actual model")
	require.Equal(t, ch.ID, result[0].Channel.ID)
	require.Len(t, result[0].Models, 1)
	require.Equal(t, "gpt-4", result[0].Models[0].ActualModel)
}

func TestDefaultSelector_Select_AggregateSameChannelSamePriority(t *testing.T) {
	ctx, client := setupTest(t)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Aggregation Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)
	selector := NewDefaultSelector(channelService, modelService, systemService)

	model, err := client.Model.Create().
		SetModelID("my-model").
		SetName("My Model").
		SetDeveloper("openai").
		SetIcon("openai").
		SetGroup("test").
		SetModelCard(&objects.ModelCard{}).
		SetStatus("enabled").
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type:     "regex",
					Priority: 1,
					Regex: &objects.RegexAssociation{
						Pattern: "gpt-4$",
					},
				},
				{
					Type:     "regex",
					Priority: 1,
					Regex: &objects.RegexAssociation{
						Pattern: "gpt-3.5-.*",
					},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	req := &llm.Request{Model: model.ModelID}
	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	require.Len(t, result, 1)
	require.Equal(t, ch.ID, result[0].Channel.ID)
	require.Equal(t, 1, result[0].Priority)
	require.Len(t, result[0].Models, 2)

	actualModels := []string{result[0].Models[0].ActualModel, result[0].Models[1].ActualModel}
	require.ElementsMatch(t, []string{"gpt-4", "gpt-3.5-turbo"}, actualModels)
}
