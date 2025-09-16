package chat

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestModelMapper_MapModel(t *testing.T) {
	ctx := context.Background()
	mapper := NewModelMapper()

	tests := []struct {
		name          string
		apiKey        *ent.APIKey
		originalModel string
		expectedModel string
	}{
		{
			name:          "nil api key",
			apiKey:        nil,
			originalModel: "gpt-4",
			expectedModel: "gpt-4",
		},
		{
			name: "no profiles",
			apiKey: &ent.APIKey{
				Name:     "test-key",
				Profiles: nil,
			},
			originalModel: "gpt-4",
			expectedModel: "gpt-4",
		},
		{
			name: "no active profile",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "",
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
			originalModel: "gpt-4",
			expectedModel: "gpt-4",
		},
		{
			name: "active profile with exact match",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name: "profile1",
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4", To: "claude-3-opus"},
							},
						},
					},
				},
			},
			originalModel: "gpt-4",
			expectedModel: "claude-3-opus",
		},
		{
			name: "active profile with wildcard match",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name: "profile1",
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-*", To: "claude-3-opus"},
							},
						},
					},
				},
			},
			originalModel: "gpt-4-turbo",
			expectedModel: "claude-3-opus",
		},
		{
			name: "active profile with regexp match",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name: "profile1",
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-.*", To: "claude-3-opus"},
							},
						},
					},
				},
			},
			originalModel: "gpt-4-turbo",
			expectedModel: "claude-3-opus",
		},
		{
			name: "active profile with no matching mapping",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name: "profile1",
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4", To: "claude-3-opus"},
							},
						},
					},
				},
			},
			originalModel: "gpt-3.5-turbo",
			expectedModel: "gpt-3.5-turbo",
		},
		{
			name: "active profile not found in profiles list",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "nonexistent",
					Profiles: []objects.APIKeyProfile{
						{
							Name: "profile1",
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4", To: "claude-3-opus"},
							},
						},
					},
				},
			},
			originalModel: "gpt-4",
			expectedModel: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapModel(ctx, tt.apiKey, tt.originalModel)
			assert.Equal(t, tt.expectedModel, result)
		})
	}
}

func TestModelMapper_MatchesMapping(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		name     string
		pattern  string
		str      string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "gpt-4",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "simple wildcard prefix",
			pattern:  "gpt-*",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "simple wildcard suffix",
			pattern:  "*-turbo",
			str:      "gpt-3.5-turbo",
			expected: true,
		},
		{
			name:     "wildcard in middle",
			pattern:  "gpt-*-turbo",
			str:      "gpt-3.5-turbo",
			expected: true,
		},
		{
			name:     "multiple wildcards",
			pattern:  "*-*-turbo",
			str:      "gpt-3.5-turbo",
			expected: true,
		},
		{
			name:     "no match",
			pattern:  "gpt-*",
			str:      "claude-3",
			expected: false,
		},
		{
			name:     "wildcard only",
			pattern:  "*",
			str:      "any-model",
			expected: true,
		},
		{
			name:     "regex special chars escaped",
			pattern:  "model.v1",
			str:      "model.v1",
			expected: true,
		},
		{
			name:     "regex special chars no match",
			pattern:  "model.v1",
			str:      "modelxv1",
			expected: true,
		},
		{
			name:     "invalid regex fallback to exact match",
			pattern:  "[invalid",
			str:      "[invalid",
			expected: true,
		},
		{
			name:     "invalid regex no match",
			pattern:  "[invalid",
			str:      "other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.matchesMapping(tt.pattern, tt.str)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestModelMapper_Cache(t *testing.T) {
	mapper := NewModelMapper()

	// Test cache functionality
	assert.Equal(t, 0, mapper.CacheSize())

	// Test pattern matching to populate cache
	mapper.matchesMapping("gpt-*", "gpt-4")
	assert.Equal(t, 1, mapper.CacheSize())

	// Test same pattern uses cache
	mapper.matchesMapping("gpt-*", "gpt-3.5")
	assert.Equal(t, 1, mapper.CacheSize())

	// Test different pattern adds to cache
	mapper.matchesMapping("claude-*", "claude-3")
	assert.Equal(t, 2, mapper.CacheSize())

	// Test exact match pattern
	mapper.matchesMapping("exact-model", "exact-model")
	assert.Equal(t, 3, mapper.CacheSize())

	// Test cache clear
	mapper.ClearCache()
	assert.Equal(t, 0, mapper.CacheSize())
}

func TestModelMapper_GetActiveProfile(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		name     string
		apiKey   *ent.APIKey
		expected *objects.APIKeyProfile
	}{
		{
			name:     "nil api key",
			apiKey:   nil,
			expected: nil,
		},
		{
			name: "no profiles",
			apiKey: &ent.APIKey{
				Profiles: nil,
			},
			expected: nil,
		},
		{
			name: "no active profile",
			apiKey: &ent.APIKey{
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
			apiKey: &ent.APIKey{
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
			result := mapper.GetActiveProfile(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestModelMapper_HasActiveProfile(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		name     string
		apiKey   *ent.APIKey
		expected bool
	}{
		{
			name:     "nil api key",
			apiKey:   nil,
			expected: false,
		},
		{
			name: "no active profile",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "",
				},
			},
			expected: false,
		},
		{
			name: "active profile with no mappings",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name:          "profile1",
							ModelMappings: []objects.ModelMapping{},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "active profile with mappings",
			apiKey: &ent.APIKey{
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
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.HasActiveProfile(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}
