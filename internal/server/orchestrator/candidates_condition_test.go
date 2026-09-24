package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestFilterResolvedCandidatesForRequest_ReasoningEffort(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{ID: 1, Type: channel.TypeOpenai}}
	for _, tc := range []struct {
		name     string
		effort   string
		operator string
		value    string
		disabled bool
		priority int
	}{
		{name: "equal", effort: "high", operator: "eq", value: "high"},
		{name: "mismatch falls back", effort: "low", operator: "eq", value: "high", priority: 1},
		{name: "missing effort falls back", operator: "eq", value: "high", priority: 1},
		{name: "not equal", effort: "low", operator: "ne", value: "high"},
		{name: "not equal mismatch", effort: "high", operator: "ne", value: "high", priority: 1},
		{name: "missing effort is not equal", operator: "ne", value: "high"},
		{name: "explicit none", effort: "none", operator: "eq", value: "none"},
		{name: "missing is not none", operator: "eq", value: "none", priority: 1},
		{name: "custom effort", effort: "max", operator: "eq", value: "max"},
		{name: "disabled condition", effort: "low", operator: "eq", value: "high", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidates := []*resolvedAssociationCandidate{
				{
					channel: ch,
					models:  []biz.ChannelModelEntry{{ActualModel: "test-model"}},
					when: &objects.ModelAssociationWhen{
						Enabled: !tc.disabled,
						Condition: &objects.Condition{
							Type:  objects.ConditionTypeGroup,
							Logic: "and",
							Conditions: []objects.Condition{{
								Type:     objects.ConditionTypeCondition,
								Field:    objects.ModelAssociationConditionFieldReasoningEffort,
								Operator: tc.operator,
								Value:    tc.value,
							}},
						},
					},
				},
				{
					channel:  ch,
					priority: 1,
					models:   []biz.ChannelModelEntry{{ActualModel: "test-model"}},
				},
			}
			got := filterResolvedCandidatesForRequest(context.Background(), &llm.Request{
				Model:           "test-model",
				ReasoningEffort: tc.effort,
			}, candidates)
			require.Len(t, got, 1)
			require.Equal(t, tc.priority, got[0].Priority)
		})
	}
}

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

	require.True(t, matchesAssociationWhen(0, false, llm.APIFormatAnthropicMessage.String(), "", requestContentFeatures{}, nil, now, when))
	require.False(t, matchesAssociationWhen(0, false, llm.APIFormatOpenAIChatCompletion.String(), "", requestContentFeatures{}, nil, now, when))
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

	require.True(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, time.Date(2026, 5, 25, 23, 30, 0, 0, time.Local), when))
	require.True(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, time.Date(2026, 5, 25, 5, 59, 0, 0, time.Local), when))
	require.False(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, time.Date(2026, 5, 25, 12, 0, 0, 0, time.Local), when))
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

	require.False(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local), when))
	require.True(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, time.Date(2026, 5, 25, 18, 0, 0, 0, time.Local), when))
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

	require.True(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{hasImage: true}, nil, now, when))
	require.False(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{hasImage: true, hasAudio: true}, nil, now, when))
	require.False(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, now, when))
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

func TestMatchesAssociationWhen_RequestHeader(t *testing.T) {
	when := &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:  objects.ConditionTypeGroup,
			Logic: "and",
			Conditions: []objects.Condition{
				{
					Type:     objects.ConditionTypeCondition,
					Field:    "request_header.X-Model",
					Operator: "eq",
					Value:    "gpt-4o",
				},
			},
		},
	}

	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local)
	headers := map[string]string{"X-Model": "gpt-4o"}

	require.True(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, headers, now, when))

	mismatched := map[string]string{"X-Model": "claude-3-7-sonnet"}
	require.False(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, mismatched, now, when))

	// Missing header evaluates to empty string -> not equal.
	require.False(t, matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, nil, now, when))
}

func TestMatchesAssociationWhen_RequestHeaderOperators(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local)
	headers := map[string]string{"User-Agent": "axonhub-cli/1.2.3"}

	cases := []struct {
		name     string
		operator string
		value    string
		want     bool
	}{
		{"contains match", "contains", "cli", true},
		{"contains no match", "contains", "browser", false},
		{"not_contains match", "not_contains", "browser", true},
		{"not_contains no match", "not_contains", "cli", false},
		{"start_with match", "start_with", "axonhub-cli", true},
		{"start_with no match", "start_with", "other", false},
		{"end_with match", "end_with", "1.2.3", true},
		{"end_with no match", "end_with", "0.0.0", false},
		{"ne match", "ne", "other", true},
		{"eq match", "eq", "axonhub-cli/1.2.3", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			when := &objects.ModelAssociationWhen{
				Enabled: true,
				Condition: &objects.Condition{
					Type:  objects.ConditionTypeGroup,
					Logic: "and",
					Conditions: []objects.Condition{
						{
							Type:     objects.ConditionTypeCondition,
							Field:    "request_header.User-Agent",
							Operator: tc.operator,
							Value:    tc.value,
						},
					},
				},
			}

			got := matchesAssociationWhen(0, false, "", "", requestContentFeatures{}, headers, now, when)
			require.Equal(t, tc.want, got)
		})
	}
}
