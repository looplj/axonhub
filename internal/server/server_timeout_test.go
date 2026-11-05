package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteTimeout(t *testing.T) {
	for _, tt := range []struct {
		name               string
		request, llm, want time.Duration
	}{
		{"unlimited LLM", 30 * time.Second, 0, 30 * time.Second},
		{"LLM longer", 30 * time.Second, 30 * time.Minute, 30 * time.Minute},
		{"ordinary longer", time.Minute, 30 * time.Second, time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{RequestTimeout: tt.request, LLMRequestTimeout: tt.llm}
			require.Equal(t, tt.want, cfg.writeTimeout())
		})
	}
}
