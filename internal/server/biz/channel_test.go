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

func TestChannelService_ListAllModels(t *testing.T) {
	tests := []struct {
		name     string
		channels []*Channel
		expected []string
	}{
		{
			name: "single channel with no mappings",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name: "single channel with model mappings",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"claude-3-opus-20240229"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "claude-3-opus", To: "claude-3-opus-20240229"},
								{From: "claude-opus", To: "claude-3-opus-20240229"},
							},
						},
					},
				},
			},
			expected: []string{"claude-3-opus-20240229", "claude-3-opus", "claude-opus"},
		},
		{
			name: "multiple channels with overlapping models",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					},
				},
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4", "gpt-4-turbo"},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"},
		},
		{
			name: "multiple channels with model mappings",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4-latest", To: "gpt-4"},
							},
						},
					},
				},
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"claude-3-opus-20240229"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "claude-3-opus", To: "claude-3-opus-20240229"},
							},
						},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-4-latest", "claude-3-opus-20240229", "claude-3-opus"},
		},
		{
			name: "mapping to unsupported model should be ignored",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4-latest", To: "gpt-4"},
								{From: "invalid-mapping", To: "unsupported-model"},
							},
						},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-4-latest"},
		},
		{
			name:     "empty channels",
			channels: []*Channel{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ChannelService{
				EnabledChannels: tt.channels,
			}

			result := svc.ListAllModels(context.Background())

			// Convert to map for easier comparison (order doesn't matter)
			resultMap := make(map[string]bool)
			for _, model := range result {
				resultMap[model.ID] = true
			}

			expectedMap := make(map[string]bool)
			for _, model := range tt.expected {
				expectedMap[model] = true
			}

			require.Equal(t, expectedMap, resultMap, "Model lists should match")
			require.Equal(t, len(tt.expected), len(result), "Should have same number of models")
		})
	}
}

func setupTestChannelService(t *testing.T) (*ChannelService, *ent.Client) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	svc := &ChannelService{
		Ent: client,
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
		input   *ent.CreateChannelInput
		wantErr bool
	}{
		{
			name: "create openai channel successfully",
			input: &ent.CreateChannelInput{
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
			input: &ent.CreateChannelInput{
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
	}

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

	// Create a test channel first
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Original Name").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "original-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
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
			id:   ch.ID,
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
			id:   ch.ID,
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
			id:   ch.ID,
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
		items         []BulkImportChannelItem
		wantSuccess   bool
		wantCreated   int
		wantFailed    int
		wantErrorsLen int
	}{
		{
			name: "import multiple channels successfully",
			items: []BulkImportChannelItem{
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
			items: []BulkImportChannelItem{
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
			items: []BulkImportChannelItem{
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
			items: []BulkImportChannelItem{
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
			items: []BulkImportChannelItem{
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
		name    string
		updates []struct {
			ID             int
			OrderingWeight int
		}
		wantErr       bool
		wantUpdated   int
		verifyWeights map[int]int
	}{
		{
			name: "update ordering weights successfully",
			updates: []struct {
				ID             int
				OrderingWeight int
			}{
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
			updates: []struct {
				ID             int
				OrderingWeight int
			}{
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
