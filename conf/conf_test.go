package conf

import (
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/server"
	"github.com/stretchr/testify/require"
)

func TestPerformanceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  server.PerformanceConfig
		wantErr bool
	}{
		{
			name: "valid config with weights summing to 1.0",
			config: server.PerformanceConfig{
				HistoricalWindow:          24 * time.Hour,
				HistoricalRefreshInterval: 5 * time.Minute,
				HistoricalWeight:          0.7,
				RealtimeWeight:            0.3,
			},
			wantErr: false,
		},
		{
			name: "valid config with equal weights",
			config: server.PerformanceConfig{
				HistoricalWindow:          1 * time.Hour,
				HistoricalRefreshInterval: 1 * time.Minute,
				HistoricalWeight:          0.5,
				RealtimeWeight:            0.5,
			},
			wantErr: false,
		},
		{
			name: "invalid config weights don't sum to 1.0",
			config: server.PerformanceConfig{
				HistoricalWindow:          24 * time.Hour,
				HistoricalRefreshInterval: 5 * time.Minute,
				HistoricalWeight:          0.5,
				RealtimeWeight:            0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid config negative historical window",
			config: server.PerformanceConfig{
				HistoricalWindow:          -1 * time.Hour,
				HistoricalRefreshInterval: 5 * time.Minute,
				HistoricalWeight:          0.7,
				RealtimeWeight:            0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid config historical weight outside [0, 1] range",
			config: server.PerformanceConfig{
				HistoricalWindow:          24 * time.Hour,
				HistoricalRefreshInterval: 5 * time.Minute,
				HistoricalWeight:          1.5,
				RealtimeWeight:            0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid config realtime weight outside [0, 1] range",
			config: server.PerformanceConfig{
				HistoricalWindow:          24 * time.Hour,
				HistoricalRefreshInterval: 5 * time.Minute,
				HistoricalWeight:          0.7,
				RealtimeWeight:            -0.2,
			},
			wantErr: true,
		},
		{
			name: "invalid config both weights outside [0, 1] range",
			config: server.PerformanceConfig{
				HistoricalWindow:          24 * time.Hour,
				HistoricalRefreshInterval: 5 * time.Minute,
				HistoricalWeight:          2.0,
				RealtimeWeight:            -1.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePerformanceConfig(tt.config)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
