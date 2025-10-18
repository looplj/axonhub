package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
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

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")

	projectService := &ProjectService{
		ProjectCache: xcache.NewFromConfig[ent.Project](cacheConfig),
	}

	apiKeyService := &APIKeyService{
		ProjectService: projectService,
		APIKeyCache:    xcache.NewFromConfig[ent.APIKey](cacheConfig),
	}

	return apiKeyService, client
}

func TestAPIKeyService_GetAPIKey(t *testing.T) {
	// Test with noop cache (no cache configured)
	cacheConfig := xcache.Config{} // Empty config = noop cache

	apiKeyService, client := setupTestAPIKeyService(t, cacheConfig)
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
		name        string
		cacheConfig xcache.Config
	}{
		{
			name:        "Memory Cache",
			cacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		},
		{
			name: "Redis Cache",
			cacheConfig: xcache.Config{
				Mode: xcache.ModeRedis,
				Redis: xcache.RedisConfig{
					Addr: miniredis.RunT(t).Addr(),
				},
			},
		},
		{
			name: "Two-Level Cache",
			cacheConfig: xcache.Config{
				Mode: xcache.ModeTwoLevel,
				Redis: xcache.RedisConfig{
					Addr: miniredis.RunT(t).Addr(),
				},
			},
		},
		{
			name:        "Noop Cache",
			cacheConfig: xcache.Config{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiKeyService, client := setupTestAPIKeyService(t, tc.cacheConfig)
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
