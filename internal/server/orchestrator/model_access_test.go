package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCheckApiKeyModelAccess(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		apiKey      *ent.APIKey
		model       string
		expectError bool
	}{
		{
			name:        "nil api key",
			apiKey:      nil,
			model:       "gpt-4",
			expectError: false,
		},
		{
			name: "no profiles",
			apiKey: &ent.APIKey{
				Name:     "test-key",
				Profiles: nil,
			},
			model:       "gpt-4",
			expectError: false,
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
						},
					},
				},
			},
			model:       "gpt-4",
			expectError: false,
		},
		{
			name: "active profile with no model restrictions",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name:     "profile1",
							ModelIDs: []string{},
						},
					},
				},
			},
			model:       "gpt-4",
			expectError: false,
		},
		{
			name: "active profile with exact match allowed",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name:     "profile1",
							ModelIDs: []string{"gpt-4"},
						},
					},
				},
			},
			model:       "gpt-4",
			expectError: false,
		},
		{
			name: "active profile with exact match denied",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name:     "profile1",
							ModelIDs: []string{"gpt-4"},
						},
					},
				},
			},
			model:       "gpt-3.5-turbo",
			expectError: true,
		},
		{
			name: "active profile with multiple exact matches",
			apiKey: &ent.APIKey{
				Name: "test-key",
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "profile1",
					Profiles: []objects.APIKeyProfile{
						{
							Name:     "profile1",
							ModelIDs: []string{"gpt-4", "claude-3-opus"},
						},
					},
				},
			},
			model:       "claude-3-opus",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PersistenceState{
				APIKey: tt.apiKey,
			}
			inbound := &PersistentInboundTransformer{
				state: state,
			}

			middleware := checkApiKeyModelAccess(inbound)

			llmRequest := &llm.Request{
				Model: tt.model,
			}

			result, err := middleware.OnInboundLlmRequest(ctx, llmRequest)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, errors.Is(err, biz.ErrInvalidModel))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.model, result.Model)
			}
		})
	}
}
