package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteTimeout(t *testing.T) {
	for _, tt := range []struct {
		name                       string
		request, llm, stream, want time.Duration
	}{
		{"unlimited LLM", 30 * time.Second, 0, 0, 30 * time.Second},
		{"LLM longer", 30 * time.Second, 30 * time.Minute, time.Minute, 30 * time.Minute},
		{"ordinary longer", time.Minute, 30 * time.Second, 30 * time.Second, time.Minute},
		{"stream longer", 30 * time.Second, 10 * time.Minute, 30 * time.Minute, 30 * time.Minute},
		{"unlimited stream", 30 * time.Second, 10 * time.Minute, 0, 10 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{RequestTimeout: tt.request, LLMRequestTimeout: tt.llm, LLMStreamRequestTimeout: tt.stream}
			require.Equal(t, tt.want, cfg.writeTimeout())
		})
	}
}
