package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
)

func TestSystemService_OnboardingInfo_NotSet(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Onboarding info not set should return nil
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.Nil(t, info)
}

func TestSystemService_OnboardingInfo_InvalidJSON(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Manually insert invalid JSON
	_, err := client.System.Create().
		SetKey(SystemKeyOnboarded).
		SetValue("invalid-json").
		Save(ctx)
	require.NoError(t, err)

	// Should return error when trying to parse invalid JSON
	_, err = service.OnboardingInfo(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal onboarding info")
}

func TestSystemService_SetOnboardingInfo(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Set onboarding info
	info := &OnboardingRecord{
		Onboarded: true,
	}

	err := service.SetOnboardingInfo(ctx, info)
	require.NoError(t, err)

	// Verify it was saved correctly
	retrievedInfo, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, retrievedInfo)
	require.True(t, retrievedInfo.Onboarded)
}

func TestSystemService_CompleteOnboarding_FirstTime(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Complete onboarding for the first time
	err := service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	// Verify onboarding is completed
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Onboarded)
	require.NotNil(t, info.CompletedAt)

	// AutoDisableChannel should also be completed for new users
	require.NotNil(t, info.AutoDisableChannel)
	require.True(t, info.AutoDisableChannel.Onboarded)
	require.NotNil(t, info.AutoDisableChannel.CompletedAt)
}

func TestSystemService_CompleteOnboarding_PreservesExistingModules(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// First, complete system model setting onboarding
	err := service.CompleteSystemModelSettingOnboarding(ctx)
	require.NoError(t, err)

	// Then complete main onboarding
	err = service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	// Verify both onboardings are completed
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Onboarded)
	require.NotNil(t, info.SystemModelSetting)
	require.True(t, info.SystemModelSetting.Onboarded)
	require.NotNil(t, info.AutoDisableChannel)
	require.True(t, info.AutoDisableChannel.Onboarded)
}

func TestSystemService_CompleteOnboarding_DoesNotOverwriteAutoDisableChannel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// First complete main onboarding (this sets AutoDisableChannel)
	err := service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	info1, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)

	firstCompletedAt := info1.AutoDisableChannel.CompletedAt

	// Complete onboarding again
	err = service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	info2, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)

	// AutoDisableChannel should not be overwritten
	require.Equal(t, firstCompletedAt, info2.AutoDisableChannel.CompletedAt)
}

func TestSystemService_CompleteSystemModelSettingOnboarding_FirstTime(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Complete system model setting onboarding
	err := service.CompleteSystemModelSettingOnboarding(ctx)
	require.NoError(t, err)

	// Verify it's completed
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, info.SystemModelSetting)
	require.True(t, info.SystemModelSetting.Onboarded)
	require.NotNil(t, info.SystemModelSetting.CompletedAt)
}

func TestSystemService_CompleteSystemModelSettingOnboarding_PreservesOtherFields(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// First complete main onboarding
	err := service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	// Then complete system model setting onboarding
	err = service.CompleteSystemModelSettingOnboarding(ctx)
	require.NoError(t, err)

	// Verify main onboarding status is preserved
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Onboarded)
	require.NotNil(t, info.CompletedAt)
	require.NotNil(t, info.AutoDisableChannel)
	require.True(t, info.AutoDisableChannel.Onboarded)
}

func TestSystemService_CompleteAutoDisableChannelOnboarding_FirstTime(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Complete auto-disable channel onboarding
	err := service.CompleteAutoDisableChannelOnboarding(ctx)
	require.NoError(t, err)

	// Verify it's completed
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, info.AutoDisableChannel)
	require.True(t, info.AutoDisableChannel.Onboarded)
	require.NotNil(t, info.AutoDisableChannel.CompletedAt)
}

func TestSystemService_CompleteAutoDisableChannelOnboarding_PreservesOtherFields(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// First complete main onboarding
	err := service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	// Then complete system model setting onboarding
	err = service.CompleteSystemModelSettingOnboarding(ctx)
	require.NoError(t, err)

	// Finally complete auto-disable channel onboarding
	err = service.CompleteAutoDisableChannelOnboarding(ctx)
	require.NoError(t, err)

	// Verify all fields are preserved
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Onboarded)
	require.NotNil(t, info.CompletedAt)
	require.NotNil(t, info.SystemModelSetting)
	require.True(t, info.SystemModelSetting.Onboarded)
	require.NotNil(t, info.AutoDisableChannel)
	require.True(t, info.AutoDisableChannel.Onboarded)
}

func TestSystemService_OnboardingInfo_FullWorkflow(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Step 1: Initially no onboarding info
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.Nil(t, info)

	// Step 2: Complete system model setting onboarding
	err = service.CompleteSystemModelSettingOnboarding(ctx)
	require.NoError(t, err)

	info, err = service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.False(t, info.Onboarded) // Main onboarding not done yet
	require.NotNil(t, info.SystemModelSetting)
	require.True(t, info.SystemModelSetting.Onboarded)
	require.Nil(t, info.AutoDisableChannel)

	// Step 3: Complete main onboarding
	err = service.CompleteOnboarding(ctx)
	require.NoError(t, err)

	info, err = service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Onboarded)
	require.NotNil(t, info.CompletedAt)
	require.NotNil(t, info.SystemModelSetting) // Still preserved
	require.True(t, info.SystemModelSetting.Onboarded)
	require.NotNil(t, info.AutoDisableChannel) // Auto-completed
	require.True(t, info.AutoDisableChannel.Onboarded)

	// Step 4: Complete auto-disable channel onboarding explicitly (should update timestamp)
	err = service.CompleteAutoDisableChannelOnboarding(ctx)
	require.NoError(t, err)

	info, err = service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, info.AutoDisableChannel)
	require.True(t, info.AutoDisableChannel.Onboarded)
}

func TestSystemService_OnboardingInfo_JSONSerialization(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Manually create a record with all fields set
	record := &OnboardingRecord{
		Onboarded:   true,
		CompletedAt: nil,
		SystemModelSetting: &OnboardingModule{
			Onboarded:   true,
			CompletedAt: nil,
		},
		AutoDisableChannel: &OnboardingModule{
			Onboarded:   false,
			CompletedAt: nil,
		},
	}

	jsonBytes, err := json.Marshal(record)
	require.NoError(t, err)

	_, err = client.System.Create().
		SetKey(SystemKeyOnboarded).
		SetValue(string(jsonBytes)).
		Save(ctx)
	require.NoError(t, err)

	// Retrieve and verify
	info, err := service.OnboardingInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Onboarded)
	require.NotNil(t, info.SystemModelSetting)
	require.True(t, info.SystemModelSetting.Onboarded)
	require.NotNil(t, info.AutoDisableChannel)
	require.False(t, info.AutoDisableChannel.Onboarded)
}
