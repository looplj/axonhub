package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelService_ListModels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create test channels with different statuses
	enabledCh, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Enabled Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	disabledCh, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Disabled Channel").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetStatus(channel.StatusDisabled).
		SetSettings(&objects.ChannelSettings{
			ModelMappings: []objects.ModelMapping{
				{From: "claude-3-opus", To: "claude-3-opus-20240229"},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	archivedCh, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Archived Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"gpt-4-turbo"}).
		SetDefaultTestModel("gpt-4-turbo").
		SetStatus(channel.StatusArchived).
		Save(ctx)
	require.NoError(t, err)

	prefixCh, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Prefix Channel").
		SetBaseURL("https://api.deepseek.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key4"}).
		SetSupportedModels([]string{"deepseek-chat", "deepseek-reasoner"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ExtraModelPrefix: "deepseek",
		}).
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name          string
		input         ListModelsInput
		wantModelIDs  []string
		wantStatuses  map[string]channel.Status
		checkStatuses bool
	}{
		{
			name: "list enabled models only (default)",
			input: ListModelsInput{
				StatusIn:       nil,
				IncludeMapping: false,
				IncludePrefix:  false,
			},
			wantModelIDs: []string{"gpt-4", "gpt-3.5-turbo", "deepseek-chat", "deepseek-reasoner"},
			wantStatuses: map[string]channel.Status{
				"gpt-4":             channel.StatusEnabled,
				"gpt-3.5-turbo":     channel.StatusEnabled,
				"deepseek-chat":     channel.StatusEnabled,
				"deepseek-reasoner": channel.StatusEnabled,
			},
			checkStatuses: true,
		},
		{
			name: "list enabled models with mappings",
			input: ListModelsInput{
				StatusIn:       []channel.Status{channel.StatusEnabled},
				IncludeMapping: true,
				IncludePrefix:  false,
			},
			wantModelIDs: []string{"gpt-4", "gpt-3.5-turbo", "deepseek-chat", "deepseek-reasoner"},
		},
		{
			name: "list enabled models with prefix",
			input: ListModelsInput{
				StatusIn:       []channel.Status{channel.StatusEnabled},
				IncludeMapping: false,
				IncludePrefix:  true,
			},
			wantModelIDs: []string{
				"gpt-4", "gpt-3.5-turbo",
				"deepseek-chat", "deepseek-reasoner",
				"deepseek/deepseek-chat", "deepseek/deepseek-reasoner",
			},
		},
		{
			name: "list disabled models with mappings",
			input: ListModelsInput{
				StatusIn:       []channel.Status{channel.StatusDisabled},
				IncludeMapping: true,
				IncludePrefix:  false,
			},
			wantModelIDs: []string{"claude-3-opus-20240229", "claude-3-opus"},
			wantStatuses: map[string]channel.Status{
				"claude-3-opus-20240229": channel.StatusDisabled,
				"claude-3-opus":          channel.StatusDisabled,
			},
			checkStatuses: true,
		},
		{
			name: "list multiple statuses",
			input: ListModelsInput{
				StatusIn:       []channel.Status{channel.StatusEnabled, channel.StatusDisabled},
				IncludeMapping: false,
				IncludePrefix:  false,
			},
			wantModelIDs: []string{
				"gpt-4", "gpt-3.5-turbo",
				"claude-3-opus-20240229",
				"deepseek-chat", "deepseek-reasoner",
			},
		},
		{
			name: "list all statuses with mappings and prefix",
			input: ListModelsInput{
				StatusIn:       []channel.Status{channel.StatusEnabled, channel.StatusDisabled, channel.StatusArchived},
				IncludeMapping: true,
				IncludePrefix:  true,
			},
			wantModelIDs: []string{
				"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo",
				"claude-3-opus-20240229", "claude-3-opus",
				"deepseek-chat", "deepseek-reasoner",
				"deepseek/deepseek-chat", "deepseek/deepseek-reasoner",
			},
		},
		{
			name: "list archived models only",
			input: ListModelsInput{
				StatusIn:       []channel.Status{channel.StatusArchived},
				IncludeMapping: false,
				IncludePrefix:  false,
			},
			wantModelIDs: []string{"gpt-4-turbo"},
			wantStatuses: map[string]channel.Status{
				"gpt-4-turbo": channel.StatusArchived,
			},
			checkStatuses: true,
		},
	}

	// Suppress unused variable warnings
	_ = enabledCh
	_ = disabledCh
	_ = archivedCh
	_ = prefixCh

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.ListModels(ctx, tt.input)
			require.NoError(t, err)

			// Extract model IDs from result
			actualIDs := lo.Map(result, func(m *ModelIdentityWithStatus, _ int) string {
				return m.ID
			})

			// Check that all expected models are present
			require.ElementsMatch(t, tt.wantModelIDs, actualIDs)

			// Check statuses if requested
			if tt.checkStatuses {
				for _, m := range result {
					expectedStatus, ok := tt.wantStatuses[m.ID]
					if ok {
						require.Equal(t, expectedStatus, m.Status, "Status mismatch for model %s", m.ID)
					}
				}
			}
		})
	}
}

func setupTestChannelService(t *testing.T) (*ChannelService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	svc := &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	return svc, client
}

func TestChannelService_CreateChannel(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	tests := []struct {
		name    string
		input   ent.CreateChannelInput
		wantErr bool
	}{
		{
			name: "create openai channel successfully",
			input: ent.CreateChannelInput{
				Type:    channel.TypeOpenai,
				Name:    "Test OpenAI Channel",
				BaseURL: lo.ToPtr("https://api.openai.com/v1"),
				Credentials: &objects.ChannelCredentials{
					APIKey: "test-api-key",
				},
				SupportedModels:  []string{"gpt-4", "gpt-3.5-turbo"},
				DefaultTestModel: "gpt-3.5-turbo",
			},
			wantErr: false,
		},
		{
			name: "create anthropic channel with settings",
			input: ent.CreateChannelInput{
				Type:    channel.TypeAnthropic,
				Name:    "Test Anthropic Channel",
				BaseURL: lo.ToPtr("https://api.anthropic.com"),
				Credentials: &objects.ChannelCredentials{
					APIKey: "test-api-key",
				},
				SupportedModels:  []string{"claude-3-opus-20240229"},
				DefaultTestModel: "claude-3-opus-20240229",
				Settings: &objects.ChannelSettings{
					ModelMappings: []objects.ModelMapping{
						{From: "claude-3-opus", To: "claude-3-opus-20240229"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "fail to create channel with duplicate name",
			input: ent.CreateChannelInput{
				Type:    channel.TypeOpenai,
				Name:    "Duplicate Channel Name",
				BaseURL: lo.ToPtr("https://api.openai.com/v1"),
				Credentials: &objects.ChannelCredentials{
					APIKey: "test-api-key",
				},
				SupportedModels:  []string{"gpt-4"},
				DefaultTestModel: "gpt-4",
			},
			wantErr: true,
		},
	}

	// Create a channel first to test duplicate name case
	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Duplicate Channel Name").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "existing-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.CreateChannel(ctx, tt.input)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.input.Name, result.Name)
				require.Equal(t, tt.input.Type, result.Type)
				require.Equal(t, *tt.input.BaseURL, result.BaseURL)
				require.Equal(t, tt.input.SupportedModels, result.SupportedModels)
				require.Equal(t, tt.input.DefaultTestModel, result.DefaultTestModel)
			}
		})
	}
}

func TestChannelService_UpdateChannel(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create test channels first
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Original Name").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "original-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	// Create second channel to test duplicate name validation
	_, err = client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Second Channel").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "second-key"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      int
		input   *ent.UpdateChannelInput
		wantErr bool
		verify  func(*testing.T, *ent.Channel)
	}{
		{
			name: "update name and base URL",
			id:   ch1.ID,
			input: &ent.UpdateChannelInput{
				Name:    lo.ToPtr("Updated Name"),
				BaseURL: lo.ToPtr("https://api.openai.com/v2"),
			},
			wantErr: false,
			verify: func(t *testing.T, result *ent.Channel) {
				require.Equal(t, "Updated Name", result.Name)
				require.Equal(t, "https://api.openai.com/v2", result.BaseURL)
			},
		},
		{
			name: "update supported models",
			id:   ch1.ID,
			input: &ent.UpdateChannelInput{
				SupportedModels: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"},
			},
			wantErr: false,
			verify: func(t *testing.T, result *ent.Channel) {
				require.ElementsMatch(t, []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"}, result.SupportedModels)
			},
		},
		{
			name: "update credentials",
			id:   ch1.ID,
			input: &ent.UpdateChannelInput{
				Credentials: &objects.ChannelCredentials{
					APIKey: "new-api-key",
				},
			},
			wantErr: false,
			verify: func(t *testing.T, result *ent.Channel) {
				require.Equal(t, "new-api-key", result.Credentials.APIKey)
			},
		},
		{
			name: "fail to update channel with duplicate name from other channel",
			id:   ch1.ID,
			input: &ent.UpdateChannelInput{
				Name: lo.ToPtr("Second Channel"),
			},
			wantErr: true,
		},
		{
			name: "update channel keeping same name",
			id:   ch1.ID,
			input: &ent.UpdateChannelInput{
				Name:    lo.ToPtr("Original Name"),
				BaseURL: lo.ToPtr("https://api.openai.com/v3"),
			},
			wantErr: false,
			verify: func(t *testing.T, result *ent.Channel) {
				require.Equal(t, "Original Name", result.Name)
				require.Equal(t, "https://api.openai.com/v3", result.BaseURL)
			},
		},
		{
			name: "update non-existent channel",
			id:   99999,
			input: &ent.UpdateChannelInput{
				Name: lo.ToPtr("Should Fail"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.UpdateChannel(ctx, tt.id, tt.input)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.verify != nil {
					tt.verify(t, result)
				}
			}
		})
	}
}

func TestChannelService_UpdateChannelStatus(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name       string
		id         int
		status     channel.Status
		wantErr    bool
		wantStatus channel.Status
	}{
		{
			name:       "disable channel",
			id:         ch.ID,
			status:     channel.StatusDisabled,
			wantErr:    false,
			wantStatus: channel.StatusDisabled,
		},
		{
			name:       "enable channel",
			id:         ch.ID,
			status:     channel.StatusEnabled,
			wantErr:    false,
			wantStatus: channel.StatusEnabled,
		},
		{
			name:    "update non-existent channel",
			id:      99999,
			status:  channel.StatusDisabled,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.UpdateChannelStatus(ctx, tt.id, tt.status)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.wantStatus, result.Status)
			}
		})
	}
}

func TestChannelService_BulkImportChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	tests := []struct {
		name          string
		items         []*BulkImportChannelItem
		wantSuccess   bool
		wantCreated   int
		wantFailed    int
		wantErrorsLen int
	}{
		{
			name: "import multiple channels successfully",
			items: []*BulkImportChannelItem{
				{
					Type:             "openai",
					Name:             "OpenAI Channel 1",
					BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
					APIKey:           lo.ToPtr("test-key-1"),
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
				{
					Type:             "anthropic",
					Name:             "Anthropic Channel 1",
					BaseURL:          lo.ToPtr("https://api.anthropic.com"),
					APIKey:           lo.ToPtr("test-key-2"),
					SupportedModels:  []string{"claude-3-opus-20240229"},
					DefaultTestModel: "claude-3-opus-20240229",
				},
			},
			wantSuccess: true,
			wantCreated: 2,
			wantFailed:  0,
		},
		{
			name: "import with invalid channel type",
			items: []*BulkImportChannelItem{
				{
					Type:             "invalid_type",
					Name:             "Invalid Channel",
					BaseURL:          lo.ToPtr("https://api.example.com"),
					APIKey:           lo.ToPtr("test-key"),
					SupportedModels:  []string{"model-1"},
					DefaultTestModel: "model-1",
				},
			},
			wantSuccess:   false,
			wantCreated:   0,
			wantFailed:    1,
			wantErrorsLen: 1,
		},
		{
			name: "import with missing base URL",
			items: []*BulkImportChannelItem{
				{
					Type:             "openai",
					Name:             "Missing BaseURL",
					BaseURL:          nil,
					APIKey:           lo.ToPtr("test-key"),
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
			},
			wantSuccess:   false,
			wantCreated:   0,
			wantFailed:    1,
			wantErrorsLen: 1,
		},
		{
			name: "import with missing API key",
			items: []*BulkImportChannelItem{
				{
					Type:             "openai",
					Name:             "Missing APIKey",
					BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
					APIKey:           nil,
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
			},
			wantSuccess:   false,
			wantCreated:   0,
			wantFailed:    1,
			wantErrorsLen: 1,
		},
		{
			name: "partial success - some valid, some invalid",
			items: []*BulkImportChannelItem{
				{
					Type:             "openai",
					Name:             "Valid Channel",
					BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
					APIKey:           lo.ToPtr("test-key"),
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
				{
					Type:             "invalid_type",
					Name:             "Invalid Channel",
					BaseURL:          lo.ToPtr("https://api.example.com"),
					APIKey:           lo.ToPtr("test-key"),
					SupportedModels:  []string{"model-1"},
					DefaultTestModel: "model-1",
				},
			},
			wantSuccess:   false,
			wantCreated:   1,
			wantFailed:    1,
			wantErrorsLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.BulkImportChannels(ctx, tt.items)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.wantSuccess, result.Success)
			require.Equal(t, tt.wantCreated, result.Created)
			require.Equal(t, tt.wantFailed, result.Failed)
			require.Len(t, result.Errors, tt.wantErrorsLen)
			require.Len(t, result.Channels, tt.wantCreated)
		})
	}
}

func TestChannelService_BulkUpdateChannelOrdering(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create test channels
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(1).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Channel 2").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetOrderingWeight(2).
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name          string
		updates       []*ChannelOrderingItem
		wantErr       bool
		wantUpdated   int
		verifyWeights map[int]int
	}{
		{
			name: "update ordering weights successfully",
			updates: []*ChannelOrderingItem{
				{ID: ch1.ID, OrderingWeight: 100},
				{ID: ch2.ID, OrderingWeight: 50},
			},
			wantErr:     false,
			wantUpdated: 2,
			verifyWeights: map[int]int{
				ch1.ID: 100,
				ch2.ID: 50,
			},
		},
		{
			name: "update with non-existent channel",
			updates: []*ChannelOrderingItem{
				{ID: 99999, OrderingWeight: 100},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.BulkUpdateChannelOrdering(ctx, tt.updates)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, result, tt.wantUpdated)

				// Verify ordering weights
				if tt.verifyWeights != nil {
					for _, ch := range result {
						expectedWeight, ok := tt.verifyWeights[ch.ID]
						if ok {
							require.Equal(t, expectedWeight, ch.OrderingWeight)
						}
					}
				}
			}
		})
	}
}

func TestChannelService_BulkCreateChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	baseURL := "https://api.openai.com/v1"

	tests := []struct {
		name             string
		existingChannels []*ent.Channel
		channelType      channel.Type
		baseName         string
		baseURL          *string
		apiKeys          []string
		supportedModels  []string
		defaultTestModel string
		wantErr          bool
		wantCreatedCount int
		wantChannelNames []string
		wantTags         []string
	}{
		{
			name:             "create multiple channels successfully",
			channelType:      channel.TypeOpenai,
			baseName:         "Test Channel",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1", "key2", "key3"},
			supportedModels:  []string{"gpt-4", "gpt-3.5-turbo"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 3,
			wantChannelNames: []string{"Test Channel - (1)", "Test Channel - (2)", "Test Channel - (3)"},
			wantTags:         []string{"Test Channel"},
		},
		{
			name: "create channels with existing base name",
			existingChannels: []*ent.Channel{
				{
					Type:             channel.TypeOpenai,
					Name:             "Existing Channel",
					BaseURL:          baseURL,
					Credentials:      &objects.ChannelCredentials{APIKey: "existing-key"},
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
			},
			channelType:      channel.TypeOpenai,
			baseName:         "Existing Channel",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1", "key2"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 2,
			wantChannelNames: []string{"Existing Channel - (1)", "Existing Channel - (2)"},
			wantTags:         []string{"Existing Channel"},
		},
		{
			name: "create channels with some existing numbered names",
			existingChannels: []*ent.Channel{
				{
					Type:             channel.TypeOpenai,
					Name:             "Test",
					BaseURL:          baseURL,
					Credentials:      &objects.ChannelCredentials{APIKey: "key0"},
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
				{
					Type:             channel.TypeOpenai,
					Name:             "Test - (1)",
					BaseURL:          baseURL,
					Credentials:      &objects.ChannelCredentials{APIKey: "key1"},
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
			},
			channelType:      channel.TypeOpenai,
			baseName:         "Test",
			baseURL:          &baseURL,
			apiKeys:          []string{"key2", "key3", "key4"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 3,
			wantChannelNames: []string{"Test - (2)", "Test - (3)", "Test - (4)"},
			wantTags:         []string{"Test"},
		},
		{
			name:             "fail with invalid channel type",
			channelType:      channel.Type("invalid-type"),
			baseName:         "Test",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          true,
			wantCreatedCount: 0,
		},
		{
			name:             "fail with no API keys",
			channelType:      channel.TypeOpenai,
			baseName:         "Test",
			baseURL:          &baseURL,
			apiKeys:          []string{},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          true,
			wantCreatedCount: 0,
		},
		{
			name:             "create single channel with numbering",
			channelType:      channel.TypeOpenai,
			baseName:         "Single Channel",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 1,
			wantChannelNames: []string{"Single Channel - (1)"},
			wantTags:         []string{"Single Channel"},
		},
		{
			name: "create single channel when numbered name exists",
			existingChannels: []*ent.Channel{
				{
					Type:             channel.TypeOpenai,
					Name:             "Conflict - (1)",
					BaseURL:          baseURL,
					Credentials:      &objects.ChannelCredentials{APIKey: "existing-key"},
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
			},
			channelType:      channel.TypeOpenai,
			baseName:         "Conflict",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 1,
			wantChannelNames: []string{"Conflict - (2)"},
			wantTags:         []string{"Conflict"},
		},
		{
			name: "create channels with gaps in numbering",
			existingChannels: []*ent.Channel{
				{
					Type:             channel.TypeOpenai,
					Name:             "Gap Test",
					BaseURL:          baseURL,
					Credentials:      &objects.ChannelCredentials{APIKey: "key0"},
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
				{
					Type:             channel.TypeOpenai,
					Name:             "Gap Test - (2)",
					BaseURL:          baseURL,
					Credentials:      &objects.ChannelCredentials{APIKey: "key2"},
					SupportedModels:  []string{"gpt-4"},
					DefaultTestModel: "gpt-4",
				},
			},
			channelType:      channel.TypeOpenai,
			baseName:         "Gap Test",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1", "key3"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 2,
			wantChannelNames: []string{"Gap Test - (1)", "Gap Test - (3)"},
			wantTags:         []string{"Gap Test"},
		},
		{
			name:             "fail with nil base URL",
			channelType:      channel.TypeOpenai,
			baseName:         "Test",
			baseURL:          nil,
			apiKeys:          []string{"key1"},
			supportedModels:  []string{"gpt-4"},
			defaultTestModel: "gpt-4",
			wantErr:          true,
			wantCreatedCount: 0,
		},
		{
			name:             "create channels with settings",
			channelType:      channel.TypeOpenai,
			baseName:         "Settings Test",
			baseURL:          &baseURL,
			apiKeys:          []string{"key1", "key2"},
			supportedModels:  []string{"gpt-4", "gpt-3.5-turbo"},
			defaultTestModel: "gpt-4",
			wantErr:          false,
			wantCreatedCount: 2,
			wantChannelNames: []string{"Settings Test - (1)", "Settings Test - (2)"},
			wantTags:         []string{"Settings Test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create existing channels if any
			for _, ch := range tt.existingChannels {
				_, err := client.Channel.Create().
					SetType(ch.Type).
					SetName(ch.Name).
					SetBaseURL(ch.BaseURL).
					SetCredentials(ch.Credentials).
					SetSupportedModels(ch.SupportedModels).
					SetDefaultTestModel(ch.DefaultTestModel).
					Save(ctx)
				require.NoError(t, err)
			}

			// Call BulkCreateChannels
			channels, err := svc.BulkCreateChannels(ctx, BulkCreateChannelsInput{
				Type:             tt.channelType,
				Name:             tt.baseName,
				Tags:             nil,
				BaseURL:          tt.baseURL,
				APIKeys:          tt.apiKeys,
				SupportedModels:  tt.supportedModels,
				DefaultTestModel: tt.defaultTestModel,
				Settings:         nil,
			})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, channels, tt.wantCreatedCount)

				// Verify channel names
				if tt.wantChannelNames != nil {
					actualNames := lo.Map(channels, func(ch *ent.Channel, _ int) string {
						return ch.Name
					})
					require.Equal(t, tt.wantChannelNames, actualNames)
				}

				// Verify tags
				if tt.wantTags != nil {
					for _, ch := range channels {
						require.Equal(t, tt.wantTags, ch.Tags)
					}
				}

				// Verify all channels have correct type and models
				for _, ch := range channels {
					require.Equal(t, tt.channelType, ch.Type)
					require.Equal(t, tt.supportedModels, ch.SupportedModels)
					require.Equal(t, tt.defaultTestModel, ch.DefaultTestModel)
					require.NotNil(t, ch.Credentials)
				}
			}

			// Clean up for next test
			_, err = client.Channel.Delete().Exec(ctx)
			require.NoError(t, err)
		})
	}
}
