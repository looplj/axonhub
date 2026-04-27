package biz

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestValidateRateLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   *objects.ChannelRateLimit
		wantErr string
	}{
		{
			name:  "nil is allowed",
			input: nil,
		},
		{
			name:  "all zero/nil is allowed",
			input: &objects.ChannelRateLimit{},
		},
		{
			name: "fully configured hard mode is allowed",
			input: &objects.ChannelRateLimit{
				RPM:            lo.ToPtr(int64(100)),
				TPM:            lo.ToPtr(int64(10000)),
				MaxConcurrent:  lo.ToPtr(int64(5)),
				QueueSize:      lo.ToPtr(int64(20)),
				QueueTimeoutMs: lo.ToPtr(int64(30000)),
			},
		},
		{
			name: "soft mode is allowed",
			input: &objects.ChannelRateLimit{
				MaxConcurrent: lo.ToPtr(int64(5)),
			},
		},
		{
			name: "negative rpm rejected",
			input: &objects.ChannelRateLimit{
				RPM: lo.ToPtr(int64(-1)),
			},
			wantErr: "rpm must be >= 0",
		},
		{
			name: "negative tpm rejected",
			input: &objects.ChannelRateLimit{
				TPM: lo.ToPtr(int64(-1)),
			},
			wantErr: "tpm must be >= 0",
		},
		{
			name: "negative maxConcurrent rejected",
			input: &objects.ChannelRateLimit{
				MaxConcurrent: lo.ToPtr(int64(-1)),
			},
			wantErr: "maxConcurrent must be >= 0",
		},
		{
			name: "negative queueSize rejected",
			input: &objects.ChannelRateLimit{
				QueueSize: lo.ToPtr(int64(-1)),
			},
			wantErr: "queueSize must be >= 0",
		},
		{
			name: "negative queueTimeoutMs rejected",
			input: &objects.ChannelRateLimit{
				QueueTimeoutMs: lo.ToPtr(int64(-1)),
			},
			wantErr: "queueTimeoutMs must be >= 0",
		},
		{
			name: "queueSize without maxConcurrent rejected",
			input: &objects.ChannelRateLimit{
				QueueSize: lo.ToPtr(int64(10)),
			},
			wantErr: "queueSize requires maxConcurrent > 0",
		},
		{
			name: "queueSize with zero maxConcurrent rejected",
			input: &objects.ChannelRateLimit{
				QueueSize:     lo.ToPtr(int64(10)),
				MaxConcurrent: lo.ToPtr(int64(0)),
			},
			wantErr: "queueSize requires maxConcurrent > 0",
		},
		{
			name: "queueTimeoutMs without queueSize is allowed (but inert)",
			input: &objects.ChannelRateLimit{
				MaxConcurrent:  lo.ToPtr(int64(5)),
				QueueTimeoutMs: lo.ToPtr(int64(1000)),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRateLimit(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
