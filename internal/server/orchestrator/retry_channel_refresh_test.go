package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// newRetryGuardChannel builds a channel snapshot for retry-guard tests.
func newRetryGuardChannel(t *testing.T, id int, keys []string, disabled []objects.DisabledAPIKey) *biz.Channel {
	t.Helper()

	outbound, err := openai.NewOutboundTransformer("https://example.com", "test-key")
	require.NoError(t, err)

	return &biz.Channel{
		Channel: &ent.Channel{
			ID:              id,
			Name:            "retry-guard",
			BaseURL:         "https://example.com",
			Credentials:     objects.ChannelCredentials{APIKeys: keys},
			DisabledAPIKeys: disabled,
		},
		Outbound: outbound,
	}
}

func TestRefreshChannelBeforeRetry(t *testing.T) {
	_, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)

	tests := []struct {
		name        string
		currentKey  string
		freshKeys   []string
		disabled    []objects.DisabledAPIKey
		channelGone bool
		wantSwapped bool
	}{
		{
			name:        "credential still enabled leaves the candidate untouched",
			currentKey:  "key-1",
			freshKeys:   []string{"key-1", "key-2"},
			wantSwapped: false,
		},
		{
			name:        "credential in use was disabled swaps to the latest snapshot",
			currentKey:  "key-1",
			freshKeys:   []string{"key-1", "key-2"},
			disabled:    []objects.DisabledAPIKey{{Key: "key-1"}},
			wantSwapped: true,
		},
		{
			name:        "a different credential being disabled does not swap",
			currentKey:  "key-2",
			freshKeys:   []string{"key-1", "key-2"},
			disabled:    []objects.DisabledAPIKey{{Key: "key-1"}},
			wantSwapped: false,
		},
		{
			name:        "expired disable does not count as disabled",
			currentKey:  "key-1",
			freshKeys:   []string{"key-1"},
			disabled:    []objects.DisabledAPIKey{{Key: "key-1", ExpiresAt: lo.ToPtr(time.Now().Add(-time.Minute))}},
			wantSwapped: false,
		},
		{
			name:        "channel absent from the enabled set leaves the candidate untouched",
			currentKey:  "key-1",
			channelGone: true,
			wantSwapped: false,
		},
		{
			name:        "oauth credential reference is skipped",
			currentKey:  objects.OAuthCredentialRef,
			freshKeys:   []string{"key-1"},
			wantSwapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := newRetryGuardChannel(t, 1, []string{"key-1", "key-2"}, nil)

			if tt.channelGone {
				channelService.SetEnabledChannelsForTest(nil)
			} else {
				channelService.SetEnabledChannelsForTest([]*biz.Channel{
					newRetryGuardChannel(t, 1, tt.freshKeys, tt.disabled),
				})
			}

			transformer := &PersistentOutboundTransformer{
				state: &PersistenceState{
					ChannelService: channelService,
					CurrentCandidate: &ChannelModelsCandidate{
						Channel: old,
						Models:  []biz.ChannelModelEntry{{ActualModel: "test-model"}},
					},
				},
			}
			transformer.wrapped = old.Outbound

			ctx := contexts.EnsureContainer(context.Background())
			ctx = contexts.WithChannelAPIKey(ctx, tt.currentKey)

			transformer.refreshChannelBeforeRetry(ctx)

			if tt.wantSwapped {
				require.NotSame(t, old, transformer.state.CurrentCandidate.Channel)
			} else {
				require.Same(t, old, transformer.state.CurrentCandidate.Channel)
			}
		})
	}
}
