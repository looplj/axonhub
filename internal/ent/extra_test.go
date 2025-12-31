package ent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/objects"
)

func TestAPIKey_GetActiveProfile(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   *APIKey
		expected *objects.APIKeyProfile
	}{
		{
			name:     "nil api key",
			apiKey:   nil,
			expected: nil,
		},
		{
			name: "no profiles",
			apiKey: &APIKey{
				Profiles: nil,
			},
			expected: nil,
		},
		{
			name: "no active profile",
			apiKey: &APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "",
					Profiles: []objects.APIKeyProfile{
						{Name: "profile1"},
					},
				},
			},
			expected: nil,
		},
		{
			name: "active profile found",
			apiKey: &APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name: "profile1",
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4", To: "claude-3"},
							},
						},
					},
				},
			},
			expected: &objects.APIKeyProfile{
				Name: "profile1",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4", To: "claude-3"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.apiKey.GetActiveProfile()
			assert.Equal(t, tt.expected, result)
		})
	}
}
