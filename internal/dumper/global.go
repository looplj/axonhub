package dumper

import (
	"context"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

var Global *Dumper

func SetGlobal(d *Dumper) {
	Global = d
}

func Enabled() bool {
	return Global != nil && Global.config.Enabled
}

func DumpStreamEvents(ctx context.Context, events []*httpclient.StreamEvent, filename string) {
	Global.DumpStreamEvents(ctx, events, filename)
}

func DumpObject(ctx context.Context, obj any, filename string) {
	Global.DumpStruct(ctx, obj, filename)
}

func DumpBytes(ctx context.Context, data []byte, filename string) {
	Global.DumpBytes(ctx, data, filename)
}
