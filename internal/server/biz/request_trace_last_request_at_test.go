package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newTraceLastRequestAtTestServices(t *testing.T) (*RequestService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:request_trace_last_request_at?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Client:        client,
	})

	requestService := NewRequestService(
		client,
		systemService.CacheConfig,
		systemService,
		usageLogService,
		dataStorageService,
		NewLiveStreamRegistry(),
	)

	return requestService, client, contexts.WithProjectID(ctx, 100)
}

func TestRequestServiceCreateRequestSetsTraceLastRequestAt(t *testing.T) {
	requestService, client, ctx := newTraceLastRequestAtTestServices(t)

	traceService := NewTraceService(TraceServiceParams{RequestService: requestService, Ent: client})
	traceEntity, err := traceService.GetOrCreateTrace(ctx, 100, "trace-last-request-at", nil)
	require.NoError(t, err)

	idleAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, client.Trace.UpdateOneID(traceEntity.ID).SetLastRequestAt(idleAt).Exec(ctx))

	req, err := requestService.CreateRequest(
		contexts.WithTrace(ctx, traceEntity),
		&llm.Request{Model: "trace-last-request-at-model"},
		&httpclient.Request{JSONBody: []byte(`{"model":"trace-last-request-at-model"}`)},
		llm.APIFormatOpenAIChatCompletion,
	)
	require.NoError(t, err)

	updated, err := client.Trace.Get(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.WithinDuration(t, req.CreatedAt, updated.LastRequestAt, time.Second)
	require.True(t, updated.LastRequestAt.After(idleAt))
}

func TestRequestServiceCreateRequestKeepsTraceLastRequestAtMonotonic(t *testing.T) {
	requestService, client, ctx := newTraceLastRequestAtTestServices(t)

	traceService := NewTraceService(TraceServiceParams{RequestService: requestService, Ent: client})
	traceEntity, err := traceService.GetOrCreateTrace(ctx, 100, "trace-last-request-at-monotonic", nil)
	require.NoError(t, err)

	// A request that reaches the update after a newer one must not move the
	// timestamp backwards.
	newerAt := time.Now().UTC().Add(time.Hour)
	require.NoError(t, client.Trace.UpdateOneID(traceEntity.ID).SetLastRequestAt(newerAt).Exec(ctx))

	_, err = requestService.CreateRequest(
		contexts.WithTrace(ctx, traceEntity),
		&llm.Request{Model: "trace-monotonic-model"},
		&httpclient.Request{JSONBody: []byte(`{"model":"trace-monotonic-model"}`)},
		llm.APIFormatOpenAIChatCompletion,
	)
	require.NoError(t, err)

	updated, err := client.Trace.Get(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.WithinDuration(t, newerAt, updated.LastRequestAt, time.Millisecond)
}

func TestRequestServiceCreateRequestWithoutTraceKeepsTracesUntouched(t *testing.T) {
	requestService, client, ctx := newTraceLastRequestAtTestServices(t)

	traceService := NewTraceService(TraceServiceParams{RequestService: requestService, Ent: client})
	traceEntity, err := traceService.GetOrCreateTrace(ctx, 100, "trace-without-request", nil)
	require.NoError(t, err)

	idleAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, client.Trace.UpdateOneID(traceEntity.ID).SetLastRequestAt(idleAt).Exec(ctx))

	_, err = requestService.CreateRequest(
		ctx,
		&llm.Request{Model: "no-trace-model"},
		&httpclient.Request{JSONBody: []byte(`{"model":"no-trace-model"}`)},
		llm.APIFormatOpenAIChatCompletion,
	)
	require.NoError(t, err)

	untouched, err := client.Trace.Get(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.True(t, untouched.LastRequestAt.Equal(idleAt))
}
