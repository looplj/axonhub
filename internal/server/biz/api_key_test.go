package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestGenerateAPIKey(t *testing.T) {
	apiKey, err := GenerateAPIKey("ah")
	require.NoError(t, err)
	require.NotEmpty(t, apiKey)
	require.True(t, len(apiKey) > 3)
	require.Equal(t, "ah-", apiKey[:3])

	// Test that multiple calls produce different keys
	apiKey2, err := GenerateAPIKey("ah")
	require.NoError(t, err)
	require.NotEqual(t, apiKey, apiKey2)
}

func setupTestAPIKeyService(t *testing.T, cacheConfig xcache.Config) (*APIKeyService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	projectService := &ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](cacheConfig),
	}

	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		CacheConfig:    cacheConfig,
		Ent:            client,
		ProjectService: projectService,
		KeyPrefix:      "ah",
	})

	return apiKeyService, client
}

func TestAPIKeyService_GetAPIKey(t *testing.T) {
	// Test with memory cache
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	apiKeyService, client := setupTestAPIKeyService(t, cacheConfig)
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a test project
	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	// Generate API key
	apiKeyString, err := GenerateAPIKey("ah")
	require.NoError(t, err)

	// Create API key in database
	apiKey, err := client.APIKey.Create().
		SetKey(apiKeyString).
		SetName("Test API Key").
		SetUser(testUser).
		SetProject(testProject).
		Save(ctx)
	require.NoError(t, err)

	// Test successful API key retrieval
	retrievedAPIKey, err := apiKeyService.GetAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	require.NotNil(t, retrievedAPIKey)
	require.Equal(t, apiKey.ID, retrievedAPIKey.ID)
	require.Equal(t, apiKey.Key, retrievedAPIKey.Key)
	require.Equal(t, apiKey.Name, retrievedAPIKey.Name)

	// Verify project is loaded in edges
	require.NotNil(t, retrievedAPIKey.Edges.Project)
	require.Equal(t, testProject.ID, retrievedAPIKey.Edges.Project.ID)
	require.Equal(t, testProject.Name, retrievedAPIKey.Edges.Project.Name)
	require.Equal(t, testProject.Status, retrievedAPIKey.Edges.Project.Status)

	// Test cache behavior - second call should still work (even with noop cache)
	retrievedAPIKey2, err := apiKeyService.GetAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	require.Equal(t, apiKey.ID, retrievedAPIKey2.ID)
	require.NotNil(t, retrievedAPIKey2.Edges.Project)
	require.Equal(t, testProject.ID, retrievedAPIKey2.Edges.Project.ID)

	// Test invalid API key
	_, err = apiKeyService.GetAPIKey(ctx, "invalid-api-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get api key")
}

func TestAPIKeyService_GetAPIKey_WithDifferentCaches(t *testing.T) {
	testCases := []struct {
		name         string
		cacheMode    string
		requireRedis bool
	}{
		{
			name:         "Memory Cache",
			cacheMode:    xcache.ModeMemory,
			requireRedis: false,
		},
		{
			name:         "Redis Cache",
			cacheMode:    xcache.ModeRedis,
			requireRedis: true,
		},
		{
			name:         "Two-Level Cache",
			cacheMode:    xcache.ModeTwoLevel,
			requireRedis: true,
		},
		{
			name:         "Noop Cache",
			cacheMode:    "",
			requireRedis: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var cacheConfig xcache.Config

			if tc.requireRedis {
				mr := miniredis.RunT(t)
				cacheConfig = xcache.Config{
					Mode: tc.cacheMode,
					Redis: xredis.Config{
						Addr: mr.Addr(),
					},
				}
			} else {
				cacheConfig = xcache.Config{Mode: tc.cacheMode}
			}

			apiKeyService, client := setupTestAPIKeyService(t, cacheConfig)
			defer apiKeyService.Stop()
			defer client.Close()

			ctx := context.Background()
			ctx = ent.NewContext(ctx, client)
			ctx = authz.WithTestBypass(ctx)

			// Create test user
			hashedPassword, err := HashPassword("test-password")
			require.NoError(t, err)

			testUser, err := client.User.Create().
				SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
				SetPassword(hashedPassword).
				SetFirstName("Test").
				SetLastName("User").
				SetStatus(user.StatusActivated).
				Save(ctx)
			require.NoError(t, err)

			// Create test project
			projectName := uuid.NewString()
			testProject, err := client.Project.Create().
				SetName(projectName).
				SetDescription(projectName).
				SetStatus(project.StatusActive).
				SetCreatedAt(time.Now()).
				SetUpdatedAt(time.Now()).
				Save(ctx)
			require.NoError(t, err)

			// Generate and create API key
			apiKeyString, err := GenerateAPIKey("ah")
			require.NoError(t, err)

			apiKey, err := client.APIKey.Create().
				SetKey(apiKeyString).
				SetName("Test API Key").
				SetUser(testUser).
				SetProject(testProject).
				Save(ctx)
			require.NoError(t, err)

			// First retrieval - should hit database
			retrievedAPIKey1, err := apiKeyService.GetAPIKey(ctx, apiKeyString)
			require.NoError(t, err)
			require.Equal(t, apiKey.ID, retrievedAPIKey1.ID)
			require.NotNil(t, retrievedAPIKey1.Edges.Project)
			require.Equal(t, testProject.ID, retrievedAPIKey1.Edges.Project.ID)

			// Second retrieval - should hit cache (if cache is enabled)
			retrievedAPIKey2, err := apiKeyService.GetAPIKey(ctx, apiKeyString)
			require.NoError(t, err)
			require.Equal(t, apiKey.ID, retrievedAPIKey2.ID)
			require.NotNil(t, retrievedAPIKey2.Edges.Project)
			require.Equal(t, testProject.ID, retrievedAPIKey2.Edges.Project.ID)

			// Update API key to invalidate cache
			_, err = apiKeyService.UpdateAPIKey(ctx, apiKey.ID, ent.UpdateAPIKeyInput{
				Name: new("Updated API Key"),
			})
			require.NoError(t, err)

			// Third retrieval - should hit database again after cache invalidation
			retrievedAPIKey3, err := apiKeyService.GetAPIKey(ctx, apiKeyString)
			require.NoError(t, err)
			require.Equal(t, apiKey.ID, retrievedAPIKey3.ID)
			require.NotNil(t, retrievedAPIKey3.Edges.Project)
			require.Equal(t, testProject.ID, retrievedAPIKey3.Edges.Project.ID)
		})
	}
}

func TestAPIKeyService_UpdateAPIKeyProfiles(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create test project
	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create UserProject relationship
	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	ctxWithUser := contexts.WithUser(ctx, testUser)

	// Create API key
	apiKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	t.Run("Valid profiles update", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "production",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
				{
					Name: "development",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5", To: "claude-2"},
					},
				},
			},
		}

		updatedAPIKey, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.NoError(t, err)
		require.NotNil(t, updatedAPIKey)
		require.NotNil(t, updatedAPIKey.Profiles)
		require.Equal(t, "production", updatedAPIKey.Profiles.ActiveProfile)
		require.Len(t, updatedAPIKey.Profiles.Profiles, 2)
	})

	t.Run("Duplicate profile names - exact match", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "production",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5", To: "claude-2"},
					},
				},
			},
		}

		_, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate profile name")
	})

	t.Run("Duplicate profile names - case insensitive", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "Production",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "Production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5", To: "claude-2"},
					},
				},
			},
		}

		_, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate profile name")
	})

	t.Run("Duplicate profile names - with whitespace", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "production",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
				{
					Name: " production ",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5", To: "claude-2"},
					},
				},
			},
		}

		_, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate profile name")
	})

	t.Run("Empty profile name", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "production",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
				{
					Name: "",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5", To: "claude-2"},
					},
				},
			},
		}

		_, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.Error(t, err)
		require.Contains(t, err.Error(), "profile name cannot be empty")
	})

	t.Run("Active profile does not exist", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "nonexistent",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
			},
		}

		_, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not exist in the profiles list")
	})

	t.Run("Invalid channel tags match mode", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "production",
			Profiles: []objects.APIKeyProfile{
				{
					Name:                 "production",
					ChannelTags:          []string{"official"},
					ChannelTagsMatchMode: objects.ChannelTagsMatchMode("invalid"),
				},
			},
		}

		_, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.Error(t, err)
		require.Contains(t, err.Error(), "channelTagsMatchMode is invalid")
	})

	t.Run("Channel tags match mode none is valid", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "production",
			Profiles: []objects.APIKeyProfile{
				{
					Name:                 "production",
					ChannelTags:          []string{"official"},
					ChannelTagsMatchMode: objects.ChannelTagsMatchModeNone,
				},
			},
		}

		updatedAPIKey, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.NoError(t, err)
		require.NotNil(t, updatedAPIKey)
		require.NotNil(t, updatedAPIKey.Profiles)
		require.Equal(t, objects.ChannelTagsMatchModeNone, updatedAPIKey.Profiles.Profiles[0].ChannelTagsMatchMode)
	})

	t.Run("Multiple profiles with unique names", func(t *testing.T) {
		profiles := objects.APIKeyProfiles{
			ActiveProfile: "staging",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "production",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-4", To: "claude-3"},
					},
				},
				{
					Name: "staging",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5", To: "claude-2"},
					},
				},
				{
					Name: "development",
					ModelMappings: []objects.ModelMapping{
						{From: "gpt-3.5-turbo", To: "claude-instant"},
					},
				},
			},
		}

		updatedAPIKey, err := apiKeyService.UpdateAPIKeyProfiles(ctx, apiKey.ID, profiles)
		require.NoError(t, err)
		require.NotNil(t, updatedAPIKey)
		require.NotNil(t, updatedAPIKey.Profiles)
		require.Equal(t, "staging", updatedAPIKey.Profiles.ActiveProfile)
		require.Len(t, updatedAPIKey.Profiles.Profiles, 3)
	})
}

func TestAPIKeyService_BulkEnableAPIKeys(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	ctxWithUser := contexts.WithUser(ctx, testUser)

	apiKey1, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 1",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	apiKey2, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 2",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	apiKey3, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 3",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	_, err = apiKeyService.UpdateAPIKeyStatus(ctx, apiKey1.ID, apikey.StatusDisabled)
	require.NoError(t, err)

	_, err = apiKeyService.UpdateAPIKeyStatus(ctx, apiKey2.ID, apikey.StatusDisabled)
	require.NoError(t, err)

	_, err = apiKeyService.UpdateAPIKeyStatus(ctx, apiKey3.ID, apikey.StatusDisabled)
	require.NoError(t, err)

	tests := []struct {
		name    string
		ids     []int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "enable multiple API keys successfully",
			ids:     []int{apiKey1.ID, apiKey2.ID},
			wantErr: false,
		},
		{
			name:    "enable single API key successfully",
			ids:     []int{apiKey3.ID},
			wantErr: false,
		},
		{
			name:    "enable with non-existent API key ID",
			ids:     []int{apiKey1.ID, 99999},
			wantErr: true,
			errMsg:  "expected to find",
		},
		{
			name:    "enable with empty list",
			ids:     []int{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apiKeyService.BulkEnableAPIKeys(ctx, tt.ids)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				if len(tt.ids) > 0 {
					for _, id := range tt.ids {
						apiKey, err := client.APIKey.Get(ctx, id)
						require.NoError(t, err)
						require.Equal(t, apikey.StatusEnabled, apiKey.Status)
					}
				}
			}
		})
	}
}

func TestAPIKeyService_BulkDisableAPIKeys(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	ctxWithUser := contexts.WithUser(ctx, testUser)

	apiKey1, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 1",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	apiKey2, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 2",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	apiKey3, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 3",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		ids     []int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "disable multiple API keys successfully",
			ids:     []int{apiKey1.ID, apiKey2.ID},
			wantErr: false,
		},
		{
			name:    "disable single API key successfully",
			ids:     []int{apiKey3.ID},
			wantErr: false,
		},
		{
			name:    "disable with non-existent API key ID",
			ids:     []int{apiKey1.ID, 99999},
			wantErr: true,
			errMsg:  "expected to find",
		},
		{
			name:    "disable with empty list",
			ids:     []int{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apiKeyService.BulkDisableAPIKeys(ctx, tt.ids)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				if len(tt.ids) > 0 {
					for _, id := range tt.ids {
						apiKey, err := client.APIKey.Get(ctx, id)
						require.NoError(t, err)
						require.Equal(t, apikey.StatusDisabled, apiKey.Status)
					}
				}
			}
		})
	}
}

func TestAPIKeyService_BulkArchiveAPIKeys(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	ctxWithUser := contexts.WithUser(ctx, testUser)

	apiKey1, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 1",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	apiKey2, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 2",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	apiKey3, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
		Name:      "Test API Key 3",
		ProjectID: testProject.ID,
	})
	require.NoError(t, err)

	_, err = apiKeyService.UpdateAPIKeyStatus(ctx, apiKey3.ID, apikey.StatusDisabled)
	require.NoError(t, err)

	tests := []struct {
		name    string
		ids     []int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "archive multiple API keys successfully",
			ids:     []int{apiKey1.ID, apiKey2.ID},
			wantErr: false,
		},
		{
			name:    "archive single API key successfully",
			ids:     []int{apiKey3.ID},
			wantErr: false,
		},
		{
			name:    "archive with non-existent API key ID",
			ids:     []int{apiKey1.ID, 99999},
			wantErr: true,
			errMsg:  "expected to find",
		},
		{
			name:    "archive with empty list",
			ids:     []int{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apiKeyService.BulkArchiveAPIKeys(ctx, tt.ids)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				if len(tt.ids) > 0 {
					for _, id := range tt.ids {
						apiKey, err := client.APIKey.Get(ctx, id)
						require.NoError(t, err)
						require.Equal(t, apikey.StatusArchived, apiKey.Status)
					}
				}
			}
		})
	}
}

func TestAPIKeyService_CreateAPIKey_Type(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	ctxWithUser := contexts.WithUser(ctx, testUser)

	t.Run("Create user type API key without specifying type (default)", func(t *testing.T) {
		apiKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "User API Key",
			ProjectID: testProject.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.Equal(t, apikey.TypeUser, apiKey.Type)
		require.NotNil(t, apiKey.Scopes)
		require.Contains(t, apiKey.Scopes, "read_channels")
		require.Contains(t, apiKey.Scopes, "write_requests")
		require.Len(t, apiKey.Scopes, 2)
	})

	t.Run("Create user type API key with explicit type", func(t *testing.T) {
		userType := apikey.TypeUser
		apiKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "User API Key Explicit",
			ProjectID: testProject.ID,
			Type:      &userType,
		})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.Equal(t, apikey.TypeUser, apiKey.Type)
		require.NotNil(t, apiKey.Scopes)
		require.Contains(t, apiKey.Scopes, "read_channels")
		require.Contains(t, apiKey.Scopes, "write_requests")
		require.Len(t, apiKey.Scopes, 2)
	})

	t.Run("Create service_account type API key without scopes", func(t *testing.T) {
		serviceAccountType := apikey.TypeServiceAccount
		apiKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Service Account API Key",
			ProjectID: testProject.ID,
			Type:      &serviceAccountType,
		})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.Equal(t, apikey.TypeServiceAccount, apiKey.Type)
		require.NotNil(t, apiKey.Scopes)
		require.Len(t, apiKey.Scopes, 0)
	})

	t.Run("Create service_account type API key with custom scopes", func(t *testing.T) {
		serviceAccountType := apikey.TypeServiceAccount
		customScopes := []string{"read_channels", "write_channels", "read_channels"}
		apiKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Service Account with Scopes",
			ProjectID: testProject.ID,
			Type:      &serviceAccountType,
			Scopes:    customScopes,
		})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.Equal(t, apikey.TypeServiceAccount, apiKey.Type)
		require.NotNil(t, apiKey.Scopes)
		require.Len(t, apiKey.Scopes, 3)
		require.Contains(t, apiKey.Scopes, "read_channels")
		require.Contains(t, apiKey.Scopes, "write_channels")
		require.Contains(t, apiKey.Scopes, "read_channels")
	})

	t.Run("Create user type API key ignores provided scopes", func(t *testing.T) {
		userType := apikey.TypeUser
		ignoredScopes := []string{"read_users", "write_channels"}
		apiKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "User API Key with Ignored Scopes",
			ProjectID: testProject.ID,
			Type:      &userType,
			Scopes:    ignoredScopes,
		})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.Equal(t, apikey.TypeUser, apiKey.Type)
		require.NotNil(t, apiKey.Scopes)
		require.Contains(t, apiKey.Scopes, "read_channels")
		require.Contains(t, apiKey.Scopes, "write_requests")
		require.Len(t, apiKey.Scopes, 2)
		require.NotContains(t, apiKey.Scopes, "read_users")
		require.NotContains(t, apiKey.Scopes, "write_channels")
	})

	t.Run("Create multiple API keys with different types", func(t *testing.T) {
		userType := apikey.TypeUser
		serviceAccountType := apikey.TypeServiceAccount

		userAPIKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "User Key",
			ProjectID: testProject.ID,
			Type:      &userType,
		})
		require.NoError(t, err)
		require.Equal(t, apikey.TypeUser, userAPIKey.Type)

		serviceAPIKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Service Key",
			ProjectID: testProject.ID,
			Type:      &serviceAccountType,
			Scopes:    []string{"read_channels"},
		})
		require.NoError(t, err)
		require.Equal(t, apikey.TypeServiceAccount, serviceAPIKey.Type)
		require.Len(t, serviceAPIKey.Scopes, 1)
		require.Contains(t, serviceAPIKey.Scopes, "read_channels")
	})

	t.Run("Verify API key key format is correct for both types", func(t *testing.T) {
		userType := apikey.TypeUser
		serviceAccountType := apikey.TypeServiceAccount

		userAPIKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "User Key for format check",
			ProjectID: testProject.ID,
			Type:      &userType,
		})
		require.NoError(t, err)
		require.True(t, len(userAPIKey.Key) > 3)
		require.Equal(t, "ah-", userAPIKey.Key[:3])

		serviceAPIKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Service Key for format check",
			ProjectID: testProject.ID,
			Type:      &serviceAccountType,
		})
		require.NoError(t, err)
		require.True(t, len(serviceAPIKey.Key) > 3)
		require.Equal(t, "ah-", serviceAPIKey.Key[:3])
		require.NotEqual(t, userAPIKey.Key, serviceAPIKey.Key)
	})
}

func TestAPIKeyService_CreateLLMAPIKey(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	// Setup context with privacy.Allow for data preparation
	setupCtx := ent.NewContext(context.Background(), client)
	setupCtx = authz.WithTestBypass(setupCtx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	ownerUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	ownerProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)

	serviceKey, err := GenerateAPIKey("ah")
	require.NoError(t, err)

	ownerAPIKey, err := client.APIKey.Create().
		SetName("Service Account").
		SetKey(serviceKey).
		SetUserID(ownerUser.ID).
		SetProjectID(ownerProject.ID).
		SetType(apikey.TypeServiceAccount).
		SetScopes([]string{string(scopes.ScopeWriteAPIKeys)}).
		Save(setupCtx)
	require.NoError(t, err)

	// Test context without privacy.Allow, using API Key for identity
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = contexts.WithAPIKey(ctx, ownerAPIKey)

	t.Run("creates llm api key", func(t *testing.T) {
		apiKey, err := apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "  LLM Key  ")
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.Equal(t, "LLM Key", apiKey.Name)
		require.Equal(t, ownerUser.ID, apiKey.UserID)
		require.Equal(t, ownerProject.ID, apiKey.ProjectID)
		require.Equal(t, apikey.TypeUser, apiKey.Type)
		require.ElementsMatch(t, []string{string(scopes.ScopeReadChannels), string(scopes.ScopeWriteRequests)}, apiKey.Scopes)
		require.NotEmpty(t, apiKey.Key)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		_, err := apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "   ")
		require.ErrorIs(t, err, ErrAPIKeyNameRequired)
	})

	t.Run("rejects unauthorized api key", func(t *testing.T) {
		// Create an API key without ScopeWriteAPIKeys
		unauthorizedKey, err := client.APIKey.Create().
			SetName("Unauthorized").
			SetKey("ah-test-unauthorized").
			SetUserID(ownerUser.ID).
			SetProjectID(ownerProject.ID).
			SetType(apikey.TypeUser).
			SetScopes([]string{string(scopes.ScopeReadChannels)}).
			Save(setupCtx)
		require.NoError(t, err)

		unauthorizedCtx := contexts.WithAPIKey(context.Background(), unauthorizedKey)
		unauthorizedCtx = ent.NewContext(unauthorizedCtx, client)

		_, err = apiKeyService.CreateLLMAPIKey(unauthorizedCtx, unauthorizedKey, "LLM Key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "deny rule")
	})
}

// TestAPIKeyService_NameUniqueness verifies application-level name uniqueness
// (Path A', no DB unique index): names are unique per project, reusable after
// soft-delete, and still occupied by archived (not deleted) keys. It drives
// CreateLLMAPIKey, whose post-insert live-count check enforces the invariant.
func TestAPIKeyService_NameUniqueness(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	setupCtx := ent.NewContext(context.Background(), client)
	setupCtx = authz.WithTestBypass(setupCtx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	ownerUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	ownerProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)

	serviceKey, err := GenerateAPIKey("ah")
	require.NoError(t, err)

	ownerAPIKey, err := client.APIKey.Create().
		SetName("Service Account").
		SetKey(serviceKey).
		SetUserID(ownerUser.ID).
		SetProjectID(ownerProject.ID).
		SetType(apikey.TypeServiceAccount).
		SetScopes([]string{string(scopes.ScopeWriteAPIKeys)}).
		Save(setupCtx)
	require.NoError(t, err)

	ctx := ent.NewContext(context.Background(), client)
	ctx = contexts.WithAPIKey(ctx, ownerAPIKey)

	t.Run("rejects duplicate name in same project", func(t *testing.T) {
		_, err := apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "dup-name")
		require.NoError(t, err)

		_, err = apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "dup-name")
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")

		// Regression: CreateLLMAPIKey inserts the row, then checks the live count
		// post-insert and returns DuplicateNameError on a collision. Because there
		// is no DB unique constraint, correctness depends on the enclosing top-level
		// transaction rolling that insert back. Assert exactly one live key keeps the
		// name, i.e. the rejected attempt left no extra live row behind.
		live, err := client.APIKey.Query().
			Where(apikey.NameEQ("dup-name"), apikey.ProjectIDEQ(ownerProject.ID)).
			Count(setupCtx)
		require.NoError(t, err)
		require.Equal(t, 1, live, "rejected duplicate create must leave no extra live row")
	})

	t.Run("name reusable after soft-delete", func(t *testing.T) {
		created, err := apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "reusable")
		require.NoError(t, err)

		// The SoftDeleteMixin hook turns Delete into setting deleted_at != 0, so the
		// composite index slot (..., deleted_at = 0) frees up.
		err = client.APIKey.DeleteOneID(created.ID).Exec(setupCtx)
		require.NoError(t, err)

		recreated, err := apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "reusable")
		require.NoError(t, err)
		require.NotEqual(t, created.ID, recreated.ID)
	})

	t.Run("archived key still occupies the name", func(t *testing.T) {
		created, err := apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "archived-name")
		require.NoError(t, err)

		// Archiving only changes status; deleted_at stays 0, so the name is still taken.
		_, err = client.APIKey.UpdateOneID(created.ID).
			SetStatus(apikey.StatusArchived).
			Save(setupCtx)
		require.NoError(t, err)

		_, err = apiKeyService.CreateLLMAPIKey(ctx, ownerAPIKey, "archived-name")
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})
}

func TestAPIKeyService_RotateAPIKey(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	// Setup context with privacy.Allow for data preparation
	setupCtx := ent.NewContext(context.Background(), client)
	setupCtx = authz.WithTestBypass(setupCtx)

	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	ownerUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().
		SetName(projectName).
		SetDescription(projectName).
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)

	ctxWithUser := contexts.WithUser(setupCtx, ownerUser)

	t.Run("rotate user type API key successfully", func(t *testing.T) {
		// Create an API key
		originalKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Key to Rotate",
			ProjectID: testProject.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, originalKey)

		originalKeyValue := originalKey.Key
		originalID := originalKey.ID
		originalName := originalKey.Name
		originalType := originalKey.Type
		originalScopes := originalKey.Scopes

		// Rotate the API key
		rotatedKey, err := apiKeyService.RotateAPIKey(ctxWithUser, originalID)
		require.NoError(t, err)
		require.NotNil(t, rotatedKey)

		// Verify the key has changed
		require.NotEqual(t, originalKeyValue, rotatedKey.Key)
		require.NotEmpty(t, rotatedKey.Key)

		// Verify ID and other properties are preserved
		require.Equal(t, originalID, rotatedKey.ID)
		require.Equal(t, originalName, rotatedKey.Name)
		require.Equal(t, originalType, rotatedKey.Type)
		require.Equal(t, originalScopes, rotatedKey.Scopes)
		require.Equal(t, testProject.ID, rotatedKey.ProjectID)
		require.Equal(t, ownerUser.ID, rotatedKey.UserID)

		// Verify the old key is no longer valid
		_, err = apiKeyService.GetAPIKey(ctxWithUser, originalKeyValue)
		require.Error(t, err)

		// Verify the new key is valid
		fetchedKey, err := apiKeyService.GetAPIKey(ctxWithUser, rotatedKey.Key)
		require.NoError(t, err)
		require.Equal(t, rotatedKey.ID, fetchedKey.ID)
	})

	t.Run("rotate service account type API key successfully", func(t *testing.T) {
		serviceAccountType := apikey.TypeServiceAccount
		customScopes := []string{"read_channels", "write_channels"}

		// Create a service account API key
		originalKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Service Key to Rotate",
			ProjectID: testProject.ID,
			Type:      &serviceAccountType,
			Scopes:    customScopes,
		})
		require.NoError(t, err)
		require.NotNil(t, originalKey)

		originalKeyValue := originalKey.Key
		originalID := originalKey.ID

		// Rotate the API key
		rotatedKey, err := apiKeyService.RotateAPIKey(ctxWithUser, originalID)
		require.NoError(t, err)
		require.NotNil(t, rotatedKey)

		// Verify the key has changed but properties are preserved
		require.NotEqual(t, originalKeyValue, rotatedKey.Key)
		require.Equal(t, originalID, rotatedKey.ID)
		require.Equal(t, apikey.TypeServiceAccount, rotatedKey.Type)
		require.Equal(t, customScopes, rotatedKey.Scopes)
	})

	t.Run("cannot rotate noauth type API key", func(t *testing.T) {
		// Create a noauth API key directly
		noauthKey, err := client.APIKey.Create().
			SetName("No Auth Key").
			SetKey("test-noauth-key-for-rotation").
			SetUserID(ownerUser.ID).
			SetProjectID(testProject.ID).
			SetType(apikey.TypeNoauth).
			SetStatus(apikey.StatusEnabled).
			Save(setupCtx)
		require.NoError(t, err)

		// Try to rotate the noauth key
		_, err = apiKeyService.RotateAPIKey(ctxWithUser, noauthKey.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "noauth type API key cannot be rotated")
	})

	t.Run("rotate non-existent API key returns error", func(t *testing.T) {
		_, err := apiKeyService.RotateAPIKey(ctxWithUser, 999999)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get API key")
	})

	t.Run("cannot rotate API key from different project", func(t *testing.T) {
		// Create another project
		anotherProject, err := client.Project.Create().
			SetName(uuid.NewString()).
			SetDescription("Another Project").
			SetStatus(project.StatusActive).
			Save(setupCtx)
		require.NoError(t, err)

		// Create another user
		anotherUser, err := client.User.Create().
			SetEmail(fmt.Sprintf("another-%d@example.com", time.Now().UnixNano())).
			SetPassword(hashedPassword).
			SetFirstName("Another").
			SetLastName("User").
			SetStatus(user.StatusActivated).
			Save(setupCtx)
		require.NoError(t, err)

		// Create an API key in the original project
		apiKeyInProjectA, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Key in Project A",
			ProjectID: testProject.ID,
		})
		require.NoError(t, err)

		// Create a context for another user without test bypass
		anotherUserCtx := ent.NewContext(context.Background(), client)
		anotherUserCtx = contexts.WithUser(anotherUserCtx, anotherUser)
		anotherUserCtx = contexts.WithProjectID(anotherUserCtx, anotherProject.ID)

		// Try to rotate the API key from project A using project B user's context
		_, err = apiKeyService.RotateAPIKey(anotherUserCtx, apiKeyInProjectA.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get API key")
	})
}
