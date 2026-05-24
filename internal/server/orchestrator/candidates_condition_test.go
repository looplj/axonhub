package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestMatchesAssociationWhen_RequestFormat(t *testing.T) {
	when := &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:  objects.ConditionTypeGroup,
			Logic: "and",
			Conditions: []objects.Condition{
				{
					Type:     objects.ConditionTypeCondition,
					Field:    "request_format",
					Operator: "eq",
					Value:    llm.APIFormatAnthropicMessage.String(),
				},
			},
		},
	}

	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local)

	require.True(t, matchesAssociationWhen(0, false, llm.APIFormatAnthropicMessage.String(), now, when))
	require.False(t, matchesAssociationWhen(0, false, llm.APIFormatOpenAIChatCompletion.String(), now, when))
}

func TestMatchesAssociationWhen_DailyTime(t *testing.T) {
	when := &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:  objects.ConditionTypeGroup,
			Logic: "and",
			Conditions: []objects.Condition{
				{
					Type:     objects.ConditionTypeCondition,
					Field:    "daily_time",
					Operator: "within",
					Value:    "22:00-06:00",
				},
			},
		},
	}

	require.True(t, matchesAssociationWhen(0, false, "", time.Date(2026, 5, 25, 23, 30, 0, 0, time.Local), when))
	require.True(t, matchesAssociationWhen(0, false, "", time.Date(2026, 5, 25, 5, 59, 0, 0, time.Local), when))
	require.False(t, matchesAssociationWhen(0, false, "", time.Date(2026, 5, 25, 12, 0, 0, 0, time.Local), when))
}

func TestMatchesAssociationWhen_DailyTimeNotWithin(t *testing.T) {
	when := &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:  objects.ConditionTypeGroup,
			Logic: "and",
			Conditions: []objects.Condition{
				{
					Type:     objects.ConditionTypeCondition,
					Field:    "daily_time",
					Operator: "not_within",
					Value:    "09:00-17:00",
				},
			},
		},
	}

	require.False(t, matchesAssociationWhen(0, false, "", time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local), when))
	require.True(t, matchesAssociationWhen(0, false, "", time.Date(2026, 5, 25, 18, 0, 0, 0, time.Local), when))
}
