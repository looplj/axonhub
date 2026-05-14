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
	newCandidate := func(model string, policy objects.CapabilityPolicy) *ChannelModelsCandidate {
		return &ChannelModelsCandidate{
			Models: []biz.ChannelModelEntry{{RequestModel: model}},
			Channel: &biz.Channel{
				Channel: &ent.Channel{
					Policies: objects.ChannelPolicies{Stream: policy},
				},
			},
		}
	}

	candidateRequestModels := func(candidates []*ChannelModelsCandidate) []string {
		return lo.Map(candidates, func(c *ChannelModelsCandidate, _ int) string {
			return c.Models[0].RequestModel
		})
	}

	tests := []struct {
		name       string
		reqStream  *bool
		reqType    llm.RequestType
		apiFormat  llm.APIFormat
		candidates []*ChannelModelsCandidate
		mockErr    error
		wantCount  int
		wantModels []string
		wantErr    bool
	}{
		{
			name:      "require stream, want stream - keep",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  1,
			wantModels: []string{"require"},
		},
		{
			name:      "require stream, no stream - keep",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  1,
			wantModels: []string{"require"},
		},
		{
			name:      "forbid stream, want stream - filter out",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				newCandidate("forbid", objects.CapabilityPolicyForbid),
			},
			wantCount:  0,
			wantModels: []string{},
		},
		{
			name:      "forbid stream, no stream - keep",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("forbid", objects.CapabilityPolicyForbid),
			},
			wantCount:  1,
			wantModels: []string{"forbid"},
		},
		{
			name:      "unlimited stream, want stream - keep",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				newCandidate("unlimited", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  1,
			wantModels: []string{"unlimited"},
		},
		{
			name:      "unlimited stream, no stream - keep",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("unlimited", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  1,
			wantModels: []string{"unlimited"},
		},
		{
			name:      "default (empty) stream policy, want stream - keep",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				newCandidate("default", ""),
			},
			wantCount:  1,
			wantModels: []string{"default"},
		},
		{
			name:      "mixed candidates, non-stream prefers native candidates only",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
				newCandidate("forbid", objects.CapabilityPolicyForbid),
				newCandidate("unlimited", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  2,
			wantModels: []string{"forbid", "unlimited"},
		},
		{
			name:      "require-only fallback is filtered for non-stream AI SDK text requests",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  0,
			wantModels: []string{},
			reqType:    llm.RequestTypeChat,
			apiFormat:  llm.APIFormatAiSDKText,
		},
		{
			name:      "require-only fallback is filtered for non-stream AI SDK data stream requests",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  0,
			wantModels: []string{},
			reqType:    llm.RequestTypeChat,
			apiFormat:  llm.APIFormatAiSDKDataStream,
		},
		{
			name:      "mixed candidates for AI SDK text keep only native candidates",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
				newCandidate("native", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  1,
			wantModels: []string{"native"},
			reqType:    llm.RequestTypeChat,
			apiFormat:  llm.APIFormatAiSDKText,
		},
		{
			name:      "mixed candidates for AI SDK data stream keep only native candidates",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
				newCandidate("native", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  1,
			wantModels: []string{"native"},
			reqType:    llm.RequestTypeChat,
			apiFormat:  llm.APIFormatAiSDKDataStream,
		},
		{
			name:      "require-only fallback stays available for non-stream chat requests",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  1,
			wantModels: []string{"require"},
		},
		{
			name:      "require-only fallback is filtered for non-stream embedding requests",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  0,
			wantModels: []string{},
			reqType:    llm.RequestTypeEmbedding,
			apiFormat:  llm.APIFormatOpenAIEmbedding,
		},
		{
			name:      "require-only fallback is filtered for non-stream compact requests",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  0,
			wantModels: []string{},
			reqType:    llm.RequestTypeCompact,
			apiFormat:  llm.APIFormatOpenAIResponseCompact,
		},
		{
			name:      "mixed candidates for supported non-stream request keep native candidates ahead of require fallback",
			reqStream: nil,
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
				newCandidate("native", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  1,
			wantModels: []string{"native"},
			reqType:    llm.RequestTypeChat,
			apiFormat:  llm.APIFormatOpenAIChatCompletion,
		},
		{
			name:      "mixed candidates for unsupported non-stream request still keep native candidates",
			reqStream: lo.ToPtr(false),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
				newCandidate("native", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  1,
			wantModels: []string{"native"},
			reqType:    llm.RequestTypeEmbedding,
			apiFormat:  llm.APIFormatOpenAIEmbedding,
		},
		{
			name:      "mixed candidates",
			reqStream: lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
				newCandidate("forbid", objects.CapabilityPolicyForbid),
				newCandidate("unlimited", objects.CapabilityPolicyUnlimited),
			},
			wantCount:  2,
			wantModels: []string{"require", "unlimited"},
		},
		{
			name:       "no candidates",
			reqStream:  lo.ToPtr(true),
			candidates: []*ChannelModelsCandidate{},
			wantCount:  0,
			wantModels: []string{},
		},
		{
			name:      "wrapped error",
			reqStream: lo.ToPtr(true),
			mockErr:   errors.New("wrapped error"),
			wantErr:   true,
		},
		{
			name:      "nil stream in request - keep require stream candidate",
			reqStream: nil,
			candidates: []*ChannelModelsCandidate{
				newCandidate("require", objects.CapabilityPolicyRequire),
			},
			wantCount:  1,
			wantModels: []string{"require"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSelector{candidates: tt.candidates, err: tt.mockErr}
			selector := WithStreamPolicySelector(mock)
			req := &llm.Request{
				Stream:      tt.reqStream,
				RequestType: tt.reqType,
				APIFormat:   tt.apiFormat,
			}

			got, err := selector.Select(context.Background(), req)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantCount)
			require.Equal(t, tt.wantModels, candidateRequestModels(got))
		})
	}
}
