package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestQuotaRoutingSettings_LegacyMapping(t *testing.T) {
	tests := []struct {
		name       string
		legacyJSON string
		want       objects.QuotaRoutingMode
	}{
		{name: "disabled", legacyJSON: `{"enabled":false}`, want: objects.QuotaRoutingModeRemoveOnExhausted},
		{name: "exhausted only", legacyJSON: `{"enabled":true,"exhaustedOnly":true}`, want: objects.QuotaRoutingModeRemoveOnExhausted},
		{name: "deprioritize", legacyJSON: `{"enabled":true,"dePrioritize":true}`, want: objects.QuotaRoutingModeBackpressure},
		{name: "absent legacy key", want: objects.QuotaRoutingModeRemoveOnExhausted},
		{name: "corrupt legacy JSON", legacyJSON: `{`, want: objects.QuotaRoutingModeRemoveOnExhausted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
			defer client.Close()

			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			if tt.legacyJSON != "" {
				_, err := client.System.Create().
					SetKey(SystemKeyQuotaEnforcementSettings).
					SetValue(tt.legacyJSON).
					Save(ctx)
				require.NoError(t, err)
			}

			service := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
			settings, err := service.QuotaRoutingSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.want, settings.DefaultMode)

			_, err = client.System.Query().Where(system.KeyEQ(SystemKeyQuotaRoutingSettings)).Only(ctx)
			require.Error(t, err)
			require.True(t, ent.IsNotFound(err))
		})
	}
}

func TestQuotaRoutingSettings_SetGetRoundTrip(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	service := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})

	want := QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeIgnoreQuota}
	require.NoError(t, service.SetQuotaRoutingSettings(ctx, want))
	got, err := service.QuotaRoutingSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, want, *got)
}

func TestQuotaRoutingSettings_InvalidModeRejected(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	service := NewSystemService(SystemServiceParams{Ent: client})

	err := service.SetQuotaRoutingSettings(ctx, QuotaRoutingSettings{DefaultMode: "invalid"})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid quota routing mode")
}
