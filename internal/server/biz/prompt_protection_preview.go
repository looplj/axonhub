package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/objects"
)

type PromptProtectionPreviewInput struct {
	Pattern  string
	TestText string
	Settings *objects.PromptProtectionSettings
}

type PromptProtectionPreviewResult struct {
	Result   string
	HasMatch bool
}

func (svc *PromptProtectionRuleService) Preview(ctx context.Context, input PromptProtectionPreviewInput) (*PromptProtectionPreviewResult, error) {
	if err := svc.ValidateSettings(input.Pattern, input.Settings); err != nil {
		return nil, err
	}

	re, err := getOrCompilePromptProtectionPattern(input.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	hasMatch, err := re.MatchString(input.TestText)
	if err != nil {
		return nil, err
	}

	result := input.TestText
	if hasMatch && input.Settings.Action == objects.PromptProtectionActionMask {
		result, err = re.Replace(input.TestText, input.Settings.Replacement, -1, -1)
		if err != nil {
			return nil, err
		}
	}

	if hasMatch && input.Settings.Action == objects.PromptProtectionActionReject {
		result = string(objects.PromptProtectionActionReject)
	}

	return &PromptProtectionPreviewResult{
		Result:   result,
		HasMatch: hasMatch,
	}, nil
}
