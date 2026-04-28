package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
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

func TestAutoReasoningEffortRunsBeforeModelAccess(t *testing.T) {
	ctx := authz.WithTestBypass(context.Background())

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)
	_, _, systemService, _ := setupTestServices(t, client)

	err := systemService.SetModelSettings(ctx, biz.SystemModelSettings{
		AutoReasoningEffort: true,
	})
	require.NoError(t, err)

	state := &PersistenceState{
		APIKey: &ent.APIKey{
			Name: "test-key",
			Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "profile1",
				Profiles: []objects.APIKeyProfile{
					{
						Name:     "profile1",
						ModelIDs: []string{"gpt-5.4"},
					},
				},
			},
		},
	}
	inbound := &PersistentInboundTransformer{
		state: state,
	}

	request := &llm.Request{
		Model:           "gpt-5.4-xhigh",
		ReasoningEffort: "low",
	}

	normalized, err := applyAutoReasoningEffort(systemService).OnInboundLlmRequest(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, normalized)
	assert.Equal(t, "gpt-5.4", normalized.Model)
	assert.Equal(t, "xhigh", normalized.ReasoningEffort)

	result, err := checkApiKeyModelAccess(inbound).OnInboundLlmRequest(ctx, normalized)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "gpt-5.4", result.Model)
	assert.Equal(t, "xhigh", result.ReasoningEffort)
}
