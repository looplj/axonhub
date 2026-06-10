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

	require.True(t, matchesAssociationWhen(0, false, llm.APIFormatAnthropicMessage.String(), requestContentFeatures{}, now, when))
	require.False(t, matchesAssociationWhen(0, false, llm.APIFormatOpenAIChatCompletion.String(), requestContentFeatures{}, now, when))
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

	require.True(t, matchesAssociationWhen(0, false, "", requestContentFeatures{}, time.Date(2026, 5, 25, 23, 30, 0, 0, time.Local), when))
	require.True(t, matchesAssociationWhen(0, false, "", requestContentFeatures{}, time.Date(2026, 5, 25, 5, 59, 0, 0, time.Local), when))
	require.False(t, matchesAssociationWhen(0, false, "", requestContentFeatures{}, time.Date(2026, 5, 25, 12, 0, 0, 0, time.Local), when))
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

	require.False(t, matchesAssociationWhen(0, false, "", requestContentFeatures{}, time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local), when))
	require.True(t, matchesAssociationWhen(0, false, "", requestContentFeatures{}, time.Date(2026, 5, 25, 18, 0, 0, 0, time.Local), when))
}

func TestMatchesAssociationWhen_ContentFeatures(t *testing.T) {
	when := &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:  objects.ConditionTypeGroup,
			Logic: "and",
			Conditions: []objects.Condition{
				{
					Type:     objects.ConditionTypeCondition,
					Field:    objects.ModelAssociationConditionFieldHasImage,
					Operator: "eq",
					Value:    true,
				},
				{
					Type:     objects.ConditionTypeCondition,
					Field:    objects.ModelAssociationConditionFieldHasAudio,
					Operator: "eq",
					Value:    false,
				},
			},
		},
	}

	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local)

	require.True(t, matchesAssociationWhen(0, false, "", requestContentFeatures{hasImage: true}, now, when))
	require.False(t, matchesAssociationWhen(0, false, "", requestContentFeatures{hasImage: true, hasAudio: true}, now, when))
	require.False(t, matchesAssociationWhen(0, false, "", requestContentFeatures{}, now, when))
}

func TestDetectRequestContentFeatures(t *testing.T) {
	req := &llm.Request{
		Messages: []llm.Message{
			{
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}},
						{Type: "video_url", VideoURL: &llm.VideoURL{URL: "https://example.com/video.mp4"}},
						{Type: "document", Document: &llm.DocumentURL{URL: "https://example.com/doc.pdf"}},
						{Type: "input_audio", InputAudio: &llm.InputAudio{Format: "mp3", Data: "audio-data"}},
					},
				},
			},
		},
	}

	features := detectRequestContentFeatures(req)

	require.True(t, features.hasImage)
	require.True(t, features.hasVideo)
	require.True(t, features.hasDocument)
	require.True(t, features.hasAudio)
}
