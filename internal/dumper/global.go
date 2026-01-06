package dumper

import (
	"context"
	"os"

	"github.com/looplj/axonhub/llm/httpclient"
)

var global *Dumper

func init() {
	if os.Getenv("AXONHUB_DEBUG_DUMPER_ENABLED") == "true" {
		// Create default config when enabled via environment variable
		config := DefaultConfig()
		config.Enabled = true

		global = New(config)
	}
}

func Enabled() bool {
	if global == nil {
		return false
	}

	return global.config.Enabled
}

func DumpStreamEvents(ctx context.Context, events []*httpclient.StreamEvent, filename string) {
	if global != nil {
		global.DumpStreamEvents(ctx, events, filename)
	}
}

func DumpObject(ctx context.Context, obj any, filename string) {
	if global != nil {
		global.DumpStruct(ctx, obj, filename)
	}
}

func DumpBytes(ctx context.Context, data []byte, filename string) {
	if global != nil {
		global.DumpBytes(ctx, data, filename)
	}
}
