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

func newTraceUpdatedAtTestServices(t *testing.T) (*RequestService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:request_trace_updated_at?mode=memory&_fk=0")
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

func TestRequestServiceCreateRequestBumpsTraceUpdatedAt(t *testing.T) {
	requestService, client, ctx := newTraceUpdatedAtTestServices(t)

	traceService := NewTraceService(TraceServiceParams{RequestService: requestService, Ent: client})
	traceEntity, err := traceService.GetOrCreateTrace(ctx, 100, "trace-updated-at", nil)
	require.NoError(t, err)

	// An idle trace keeps the timestamp of its previous activity.
	staleUpdatedAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, client.Trace.UpdateOneID(traceEntity.ID).SetUpdatedAt(staleUpdatedAt).Exec(ctx))

	req, err := requestService.CreateRequest(
		contexts.WithTrace(ctx, traceEntity),
		&llm.Request{Model: "trace-updated-at-model"},
		&httpclient.Request{JSONBody: []byte(`{"model":"trace-updated-at-model"}`)},
		llm.APIFormatOpenAIChatCompletion,
	)
	require.NoError(t, err)

	updated, err := client.Trace.Get(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.True(t, updated.UpdatedAt.After(staleUpdatedAt), "trace updated_at should move forward on a new request")
	require.WithinDuration(t, req.CreatedAt, updated.UpdatedAt, time.Second)
}

func TestRequestServiceCreateRequestWithoutTraceKeepsTracesUntouched(t *testing.T) {
	requestService, client, ctx := newTraceUpdatedAtTestServices(t)

	traceService := NewTraceService(TraceServiceParams{RequestService: requestService, Ent: client})
	traceEntity, err := traceService.GetOrCreateTrace(ctx, 100, "trace-without-request", nil)
	require.NoError(t, err)

	staleUpdatedAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, client.Trace.UpdateOneID(traceEntity.ID).SetUpdatedAt(staleUpdatedAt).Exec(ctx))

	_, err = requestService.CreateRequest(
		ctx,
		&llm.Request{Model: "no-trace-model"},
		&httpclient.Request{JSONBody: []byte(`{"model":"no-trace-model"}`)},
		llm.APIFormatOpenAIChatCompletion,
	)
	require.NoError(t, err)

	untouched, err := client.Trace.Get(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.True(t, untouched.UpdatedAt.Equal(staleUpdatedAt))
}
