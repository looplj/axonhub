package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestGenerateAPIKey(t *testing.T) {
	apiKey, err := GenerateAPIKey()
	require.NoError(t, err)
	require.NotEmpty(t, apiKey)
	require.True(t, len(apiKey) > 3)
	require.Equal(t, "ah-", apiKey[:3])

	// Test that multiple calls produce different keys
	apiKey2, err := GenerateAPIKey()
	require.NoError(t, err)
	require.NotEqual(t, apiKey, apiKey2)
}

func setupTestAPIKeyService(t *testing.T, cacheConfig xcache.Config) (*APIKeyService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	projectService := &ProjectService{
		ProjectCache: xcache.NewFromConfig[ent.Project](cacheConfig),
	}

	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		CacheConfig:    cacheConfig,
		Ent:            client,
		ProjectService: projectService,
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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	apiKeyString, err := GenerateAPIKey()
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
			ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
			apiKeyString, err := GenerateAPIKey()
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
				Name: stringPtr("Updated API Key"),
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

func stringPtr(s string) *string {
	return &s
}

func TestAPIKeyService_UpdateAPIKeyProfiles(t *testing.T) {
	apiKeyService, client := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer apiKeyService.Stop()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
			Name:      "User Key",
			ProjectID: testProject.ID,
			Type:      &userType,
		})
		require.NoError(t, err)
		require.True(t, len(userAPIKey.Key) > 3)
		require.Equal(t, "ah-", userAPIKey.Key[:3])

		serviceAPIKey, err := apiKeyService.CreateAPIKey(ctxWithUser, ent.CreateAPIKeyInput{
			Name:      "Service Key",
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
	setupCtx := context.Background()
	setupCtx = ent.NewContext(setupCtx, client)
	setupCtx = privacy.DecisionContext(setupCtx, privacy.Allow)

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

	serviceKey, err := GenerateAPIKey()
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
