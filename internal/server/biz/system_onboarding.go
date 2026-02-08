package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

// OnboardingModule represents a granular onboarding module.
type OnboardingModule struct {
	Onboarded   bool       `json:"onboarded"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// OnboardingRecord represents the onboarding status information.
type OnboardingRecord struct {
	Onboarded   bool       `json:"onboarded"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	SystemModelSetting *OnboardingModule `json:"system_model_setting,omitempty"`

	AutoDisableChannel *OnboardingModule `json:"auto_disable_channel,omitempty"`
}

// OnboardingInfo retrieves the onboarding information from system settings.
// Returns nil if not set.
func (s *SystemService) OnboardingInfo(ctx context.Context) (*OnboardingRecord, error) {
	ctx = authz.WithSystemBypass(ctx, "read-onboarding-info")

	value, err := s.getSystemValue(ctx, SystemKeyOnboarded)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get onboarding info: %w", err)
	}

	var info OnboardingRecord
	if err := json.Unmarshal([]byte(value), &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal onboarding info: %w", err)
	}

	return &info, nil
}

// SetOnboardingInfo sets the onboarding information.
func (s *SystemService) SetOnboardingInfo(ctx context.Context, info *OnboardingRecord) error {
	jsonBytes, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal onboarding info: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyOnboarded, string(jsonBytes))
}

// CompleteOnboarding marks onboarding as completed.
// For new users, this also marks AutoDisableChannel as completed since they see it in the main flow.
func (s *SystemService) CompleteOnboarding(ctx context.Context) error {
	info, err := s.OnboardingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing onboarding info: %w", err)
	}

	if info == nil {
		info = &OnboardingRecord{}
	}

	now := time.Now()
	info.Onboarded = true
	info.CompletedAt = &now

	// For new users, this also marks AutoDisableChannel as completed since they see it in the main flow.
	if info.AutoDisableChannel == nil {
		info.AutoDisableChannel = &OnboardingModule{
			Onboarded:   true,
			CompletedAt: &now,
		}
	}

	return s.SetOnboardingInfo(ctx, info)
}

// CompleteSystemModelSettingOnboarding marks system model setting onboarding as completed.
func (s *SystemService) CompleteSystemModelSettingOnboarding(ctx context.Context) error {
	info, err := s.OnboardingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing onboarding info: %w", err)
	}

	if info == nil {
		info = &OnboardingRecord{}
	}

	now := time.Now()
	info.SystemModelSetting = &OnboardingModule{
		Onboarded:   true,
		CompletedAt: &now,
	}

	return s.SetOnboardingInfo(ctx, info)
}

// CompleteAutoDisableChannelOnboarding marks auto-disable channel onboarding as completed.
func (s *SystemService) CompleteAutoDisableChannelOnboarding(ctx context.Context) error {
	info, err := s.OnboardingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing onboarding info: %w", err)
	}

	if info == nil {
		info = &OnboardingRecord{}
	}

	now := time.Now()
	info.AutoDisableChannel = &OnboardingModule{
		Onboarded:   true,
		CompletedAt: &now,
	}

	return s.SetOnboardingInfo(ctx, info)
}
