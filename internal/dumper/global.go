package dumper

import (
	"context"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

var Global *Dumper

func SetGlobal(d *Dumper) {
	Global = d
}

func DumpStreamEvents(ctx context.Context, events []*httpclient.StreamEvent, filename string) {
	Global.DumpStreamEvents(ctx, events, filename)
}

func DumpStruct(ctx context.Context, obj any, filename string) {
	Global.DumpStruct(ctx, obj, filename)
}
