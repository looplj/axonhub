package biz

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestProjectService(t *testing.T, cacheConfig xcache.Config) (*ProjectService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")

	projectService := &ProjectService{
		ProjectCache: xcache.NewFromConfig[ent.Project](cacheConfig),
	}

	return projectService, client
}

func TestProjectService_GetProjectByID(t *testing.T) {
	// Test with noop cache (no cache configured)
	cacheConfig := xcache.Config{} // Empty config = noop cache

	projectService, client := setupTestProjectService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

	// Test successful project retrieval
	retrievedProject, err := projectService.GetProjectByID(ctx, testProject.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedProject)
	require.Equal(t, testProject.ID, retrievedProject.ID)
	require.Equal(t, testProject.Name, retrievedProject.Name)
	require.Equal(t, testProject.Status, retrievedProject.Status)

	// Test cache behavior - second call should still work (even with noop cache)
	retrievedProject2, err := projectService.GetProjectByID(ctx, testProject.ID)
	require.NoError(t, err)
	require.Equal(t, testProject.ID, retrievedProject2.ID)

	// Test invalid project ID
	_, err = projectService.GetProjectByID(ctx, 99999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get project")
}

func TestProjectService_GetProjectByID_WithDifferentCaches(t *testing.T) {
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
			projectService, client := setupTestProjectService(t, tc.cacheConfig)
			defer client.Close()

			ctx := context.Background()
			ctx = ent.NewContext(ctx, client)
			ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

			// First retrieval - should hit database
			retrievedProject1, err := projectService.GetProjectByID(ctx, testProject.ID)
			require.NoError(t, err)
			require.Equal(t, testProject.ID, retrievedProject1.ID)
			require.Equal(t, testProject.Name, retrievedProject1.Name)

			// Second retrieval - should hit cache (if cache is enabled)
			retrievedProject2, err := projectService.GetProjectByID(ctx, testProject.ID)
			require.NoError(t, err)
			require.Equal(t, testProject.ID, retrievedProject2.ID)
			require.Equal(t, testProject.Name, retrievedProject2.Name)

			// Update project to invalidate cache
			newName := "Updated " + projectName
			_, err = projectService.UpdateProject(ctx, testProject.ID, ent.UpdateProjectInput{
				Name: &newName,
			})
			require.NoError(t, err)

			// Third retrieval - should hit database again after cache invalidation
			retrievedProject3, err := projectService.GetProjectByID(ctx, testProject.ID)
			require.NoError(t, err)
			require.Equal(t, testProject.ID, retrievedProject3.ID)
			require.Equal(t, newName, retrievedProject3.Name)
		})
	}
}

func TestProjectService_UpdateProjectStatus_CacheInvalidation(t *testing.T) {
	// Test with memory cache
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	projectService, client := setupTestProjectService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

	// First retrieval - populate cache
	retrievedProject1, err := projectService.GetProjectByID(ctx, testProject.ID)
	require.NoError(t, err)
	require.Equal(t, project.StatusActive, retrievedProject1.Status)

	// Update project status
	_, err = projectService.UpdateProjectStatus(ctx, testProject.ID, project.StatusArchived)
	require.NoError(t, err)

	// Second retrieval - should get updated status from database (cache invalidated)
	retrievedProject2, err := projectService.GetProjectByID(ctx, testProject.ID)
	require.NoError(t, err)
	require.Equal(t, project.StatusArchived, retrievedProject2.Status)
}

func TestBuildProjectCacheKey(t *testing.T) {
	testCases := []struct {
		name     string
		id       int
		expected string
	}{
		{
			name:     "ID 1",
			id:       1,
			expected: "project:1",
		},
		{
			name:     "ID 123",
			id:       123,
			expected: "project:123",
		},
		{
			name:     "ID 999999",
			id:       999999,
			expected: "project:999999",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildProjectCacheKey(tc.id)
			require.Equal(t, tc.expected, result)
		})
	}
}
