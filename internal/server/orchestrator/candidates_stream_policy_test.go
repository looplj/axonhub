package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type mockSelector struct {
	candidates []*ChannelModelsCandidate
	err        error
}

func (m *mockSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	return m.candidates, m.err
}

func TestStreamPolicySelector_Select(t *testing.T) {
	tests := []struct {
		name       string
		reqStream  *bool
		candidates []*ChannelModelsCandidate
		mockErr    error
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "require stream, want stream - keep",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyRequire,
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:      "require stream, no stream - filter out",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyRequire,
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name:      "forbid stream, want stream - filter out",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyForbid,
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name:      "forbid stream, no stream - keep",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyForbid,
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:      "unlimited stream, want stream - keep",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyUnlimited,
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:      "unlimited stream, no stream - keep",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyUnlimited,
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:      "default (empty) stream policy, want stream - keep",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:      "mixed candidates",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{Stream: objects.CapabilityPolicyRequire},
						},
					},
				},
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{Stream: objects.CapabilityPolicyForbid},
						},
					},
				},
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{Stream: objects.CapabilityPolicyUnlimited},
						},
					},
				},
			},
			wantCount: 2, // Require and Unlimited should be kept
		},
		{
			name:       "no candidates",
			reqStream:  lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{},
			wantCount:  0,
		},
		{
			name:      "wrapped error",
			reqStream: lo.ToPtr(true),
			mockErr:   errors.New("wrapped error"),
			wantErr:   true,
		},
		{
			name:      "nil stream in request - treated as false",
			reqStream: nil,
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Stream: objects.CapabilityPolicyRequire,
							},
						},
					},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSelector{candidates: tt.candidates, err: tt.mockErr}
			selector := WithStreamPolicySelector(mock)
			req := &llm.Request{Stream: tt.reqStream}

			got, err := selector.Select(context.Background(), req)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantCount)
		})
	}
}
