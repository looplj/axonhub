package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestGetHttpClientWithTLSFingerprint(t *testing.T) {
	tests := []struct {
		name                 string
		channelMode          *string
		globalMode           *string
		globalTLSFingerprint *bool
		expectTLSEnabled     bool
	}{
		{
			name:                 "mode=nil + global TLSFingerprint=true => no TLS fingerprint",
			channelMode:          nil,
			globalMode:           nil,
			globalTLSFingerprint: boolPtr(true),
			expectTLSEnabled:     false,
		},
		{
			name:                 "follow_global + global mode=auto + TLSFingerprint=nil => default false",
			channelMode:          strPtr("follow_global"),
			globalMode:           strPtr("auto"),
			globalTLSFingerprint: nil,
			expectTLSEnabled:     false,
		},
		{
			name:                 "follow_global + global mode=auto + TLSFingerprint=true => enabled",
			channelMode:          strPtr("follow_global"),
			globalMode:           strPtr("auto"),
			globalTLSFingerprint: boolPtr(true),
			expectTLSEnabled:     true,
		},
		{
			name:                 "channel mode=never overrides global auto/true => disabled",
			channelMode:          strPtr("never"),
			globalMode:           strPtr("auto"),
			globalTLSFingerprint: boolPtr(true),
			expectTLSEnabled:     false,
		},
		{
			name:                 "channel mode=auto + global TLSFingerprint=true => enabled",
			channelMode:          strPtr("auto"),
			globalMode:           nil,
			globalTLSFingerprint: boolPtr(true),
			expectTLSEnabled:     true,
		},
		{
			name:                 "channel mode=always + global TLSFingerprint=true => enabled",
			channelMode:          strPtr("always"),
			globalMode:           nil,
			globalTLSFingerprint: boolPtr(true),
			expectTLSEnabled:     true,
		},
		{
			name:                 "channel mode=auto + global TLSFingerprint=false => disabled",
			channelMode:          strPtr("auto"),
			globalMode:           nil,
			globalTLSFingerprint: boolPtr(false),
			expectTLSEnabled:     false,
		},
		{
			name:                 "channel mode=nil + global mode=never => disabled",
			channelMode:          nil,
			globalMode:           strPtr("never"),
			globalTLSFingerprint: boolPtr(true),
			expectTLSEnabled:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
			defer client.Close()

			ctx := context.Background()
			ctx = authz.WithSystemBypass(ctx, "test")

			// Create system service and set global cloaking config
			systemSvc := NewSystemService(SystemServiceParams{
				CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
				Ent:         client,
			})
			if tt.globalMode != nil || tt.globalTLSFingerprint != nil {
				err := systemSvc.SetGlobalCloakingConfig(ctx, &GlobalCloakingConfig{
					Mode:           tt.globalMode,
					TLSFingerprint: tt.globalTLSFingerprint,
				})
				require.NoError(t, err)
			}

			// Create channel service
			channelSvc := &ChannelService{
				AbstractService: &AbstractService{
				db: client,
				},
				SystemService: systemSvc,
				httpClient:    httpclient.NewHttpClient(),
			}

			// Create test channel
			ch := client.Channel.Create().
				SetName("test-channel").
				SetType(channel.TypeAnthropic).
				SetBaseURL("https://api.anthropic.com").
				SetCredentials(objects.ChannelCredentials{
					APIKeys: []string{"test-key"},
				}).
				SetSupportedModels([]string{"claude-3-5-sonnet-20241022"}).
				SetDefaultTestModel("claude-3-5-sonnet-20241022").
				SetSettings(&objects.ChannelSettings{
					CloakingMode: tt.channelMode,
				}).
				SaveX(ctx)

			// Get HTTP client
			httpClient := channelSvc.getHttpClient(ctx, ch)
			require.NotNil(t, httpClient)

			// Check if TLS fingerprint is enabled by inspecting the client's internal state
			// Since we can't directly check the internal state, we verify through behavior
			// The WithTLSFingerprint method returns a new client with TLS fingerprint enabled
			if tt.expectTLSEnabled {
				assert.NotEqual(t, channelSvc.httpClient, httpClient, "Expected new client with TLS fingerprint")
			} else {
				// When TLS is not enabled and no proxy, should return the default client
				if ch.Settings.Proxy == nil {
					assert.Equal(t, channelSvc.httpClient, httpClient, "Expected default client without TLS fingerprint")
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
