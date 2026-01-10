package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/objects"
)

func TestBackupService_Backup_WithAPIKeys(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	require.NotNil(t, user)

	proj1 := createBackupTestProject(t, client, ctx, "Project1", "Test Project 1")
	proj2 := createBackupTestProject(t, client, ctx, "Project2", "Test Project 2")

	ak1 := createBackupTestAPIKey(t, client, ctx, user, proj1, "API Key 1", "sk-test-key-1")
	ak2 := createBackupTestAPIKey(t, client, ctx, user, proj2, "API Key 2", "sk-test-key-2")

	data, err := service.Backup(ctx, BackupOptions{
		IncludeAPIKeys: true,
	})
	require.NoError(t, err)
	require.NotNil(t, data)

	var backupData BackupData

	err = json.Unmarshal(data, &backupData)
	require.NoError(t, err)

	require.Equal(t, BackupVersion, backupData.Version)
	require.Len(t, backupData.APIKeys, 2)

	require.Equal(t, ak1.Name, backupData.APIKeys[0].Name)
	require.Equal(t, ak1.Key, backupData.APIKeys[0].Key)
	require.Equal(t, "Project1", backupData.APIKeys[0].ProjectName)

	require.Equal(t, ak2.Name, backupData.APIKeys[1].Name)
	require.Equal(t, ak2.Key, backupData.APIKeys[1].Key)
	require.Equal(t, "Project2", backupData.APIKeys[1].ProjectName)
}

func TestBackupService_Restore_APIKeys_NewKeys(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	proj1 := createBackupTestProject(t, client, ctx, "TestProject", "Test Project")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-new-key-1",
					Name:     "New API Key 1",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "TestProject",
			},
			{
				APIKey: ent.APIKey{
					Key:      "sk-new-key-2",
					Name:     "New API Key 2",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "TestProject",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	apiKeys, err := client.APIKey.Query().WithProject().All(ctx)
	require.NoError(t, err)
	require.Len(t, apiKeys, 2)

	ak1, err := client.APIKey.Query().
		Where(apikey.Key("sk-new-key-1")).
		WithProject().
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "New API Key 1", ak1.Name)
	require.Equal(t, proj1.ID, ak1.Edges.Project.ID)
	require.Equal(t, "TestProject", ak1.Edges.Project.Name)

	ak2, err := client.APIKey.Query().
		Where(apikey.Key("sk-new-key-2")).
		WithProject().
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "New API Key 2", ak2.Name)
	require.Equal(t, proj1.ID, ak2.Edges.Project.ID)
}

func TestBackupService_Restore_APIKeys_DefaultProject(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	createBackupTestProject(t, client, ctx, "Default", "Default Project")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-default-key",
					Name:     "Default API Key",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	ak, err := client.APIKey.Query().
		Where(apikey.Key("sk-default-key")).
		WithProject().
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Default API Key", ak.Name)
	require.Equal(t, "Default", ak.Edges.Project.Name)
}

func TestBackupService_Restore_APIKeys_ProjectNotFound(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-orphan-key",
					Name:     "Orphan API Key",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "NonExistentProject",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	count, err := client.APIKey.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestBackupService_Restore_APIKeys_ConflictSkip(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	proj := createBackupTestProject(t, client, ctx, "TestProject", "Test Project")
	createBackupTestAPIKey(t, client, ctx, user, proj, "Existing Key", "sk-existing-key")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-existing-key",
					Name:     "Updated Key Name",
					Type:     "service_account",
					Status:   "disabled",
					Scopes:   []string{"chat", "embedding"},
					Profiles: profiles,
				},
				ProjectName: "TestProject",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategySkip,
	})
	require.NoError(t, err)

	ak, err := client.APIKey.Query().
		Where(apikey.Key("sk-existing-key")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Existing Key", ak.Name)
	require.Equal(t, apikey.TypeUser, ak.Type)
	require.Equal(t, apikey.StatusEnabled, ak.Status)
}

func TestBackupService_Restore_APIKeys_ConflictOverwrite(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	proj := createBackupTestProject(t, client, ctx, "TestProject", "Test Project")
	createBackupTestAPIKey(t, client, ctx, user, proj, "Existing Key", "sk-existing-key")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "production",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "production",
				ModelIDs: []string{"gpt-4", "claude-3"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-existing-key",
					Name:     "Updated Key Name",
					Type:     "service_account",
					Status:   "disabled",
					Scopes:   []string{"chat", "embedding"},
					Profiles: profiles,
				},
				ProjectName: "TestProject",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	ak, err := client.APIKey.Query().
		Where(apikey.Key("sk-existing-key")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Updated Key Name", ak.Name)
	require.Equal(t, apikey.TypeServiceAccount, ak.Type)
	require.Equal(t, apikey.StatusDisabled, ak.Status)
	require.Equal(t, []string{"chat", "embedding"}, ak.Scopes)
	require.Equal(t, "production", ak.Profiles.ActiveProfile)
}

func TestBackupService_Restore_APIKeys_ConflictError(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	proj := createBackupTestProject(t, client, ctx, "TestProject", "Test Project")
	createBackupTestAPIKey(t, client, ctx, user, proj, "Existing Key", "sk-existing-key")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-existing-key",
					Name:     "Updated Key Name",
					Type:     "service_account",
					Status:   "disabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "TestProject",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyError,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestBackupService_Restore_APIKeys_MultipleProjects(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	proj1 := createBackupTestProject(t, client, ctx, "Project1", "Test Project 1")
	proj2 := createBackupTestProject(t, client, ctx, "Project2", "Test Project 2")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-proj1-key",
					Name:     "Project 1 Key",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "Project1",
			},
			{
				APIKey: ent.APIKey{
					Key:      "sk-proj2-key",
					Name:     "Project 2 Key",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
				ProjectName: "Project2",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	ak1, err := client.APIKey.Query().
		Where(apikey.Key("sk-proj1-key")).
		WithProject().
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Project 1 Key", ak1.Name)
	require.Equal(t, proj1.ID, ak1.Edges.Project.ID)

	ak2, err := client.APIKey.Query().
		Where(apikey.Key("sk-proj2-key")).
		WithProject().
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Project 2 Key", ak2.Name)
	require.Equal(t, proj2.ID, ak2.Edges.Project.ID)
}
