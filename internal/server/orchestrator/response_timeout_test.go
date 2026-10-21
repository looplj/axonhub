package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func newResponseTimeoutTransformer(settings *objects.ChannelSettings) *PersistentOutboundTransformer {
	return &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: &biz.Channel{Channel: &ent.Channel{Settings: settings}},
			},
		},
	}
}

func TestPersistentOutboundTransformer_ResponseTimeouts(t *testing.T) {
	tests := []struct {
		name          string
		settings      *objects.ChannelSettings
		wantStream    *time.Duration
		wantNonStream *time.Duration
	}{
		{
			name:     "nil settings inherits",
			settings: nil,
		},
		{
			name:     "empty settings inherits",
			settings: &objects.ChannelSettings{},
		},
		{
			name: "nil response timeout inherits",
			settings: &objects.ChannelSettings{
				ResponseTimeout: nil,
			},
		},
		{
			name: "inherit mode ignores stored values",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                            objects.ChannelResponseTimeoutModeInherit,
					StreamFirstEventTimeoutSeconds:  lo.ToPtr(15),
					NonStreamResponseTimeoutSeconds: lo.ToPtr(25),
				},
			},
		},
		{
			name: "custom overrides both timeouts",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                            objects.ChannelResponseTimeoutModeCustom,
					StreamFirstEventTimeoutSeconds:  lo.ToPtr(15),
					NonStreamResponseTimeoutSeconds: lo.ToPtr(25),
				},
			},
			wantStream:    lo.ToPtr(15 * time.Second),
			wantNonStream: lo.ToPtr(25 * time.Second),
		},
		{
			name: "custom overrides only the stream timeout",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                           objects.ChannelResponseTimeoutModeCustom,
					StreamFirstEventTimeoutSeconds: lo.ToPtr(15),
				},
			},
			wantStream: lo.ToPtr(15 * time.Second),
		},
		{
			name: "custom zero disables the timeout",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                            objects.ChannelResponseTimeoutModeCustom,
					StreamFirstEventTimeoutSeconds:  lo.ToPtr(0),
					NonStreamResponseTimeoutSeconds: lo.ToPtr(0),
				},
			},
			wantStream:    lo.ToPtr(time.Duration(0)),
			wantNonStream: lo.ToPtr(time.Duration(0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := newResponseTimeoutTransformer(tt.settings)

			streamTimeout, nonStreamTimeout := transformer.ResponseTimeouts(context.Background())
			require.Equal(t, tt.wantStream, streamTimeout)
			require.Equal(t, tt.wantNonStream, nonStreamTimeout)
		})
	}
}

func TestPersistentOutboundTransformer_ResponseTimeouts_NilCandidate(t *testing.T) {
	transformer := &PersistentOutboundTransformer{state: &PersistenceState{}}

	streamTimeout, nonStreamTimeout := transformer.ResponseTimeouts(context.Background())
	require.Nil(t, streamTimeout)
	require.Nil(t, nonStreamTimeout)
}
