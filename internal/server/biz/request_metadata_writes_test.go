package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

// The status and finalize writers read only metadata columns and do not read the updated row
// back; this checks they still persist every field and leave the stored payloads intact.
func TestRequestWritersPreservePayloadColumns(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request-metadata-writes?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	system := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	storage := NewDataStorageService(DataStorageServiceParams{Client: client, SystemService: system, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	svc := NewRequestService(client, system.CacheConfig, system, nil, storage, NewLiveStreamRegistry())

	policy := defaultStoragePolicy
	policy.StoreChunks = true
	require.NoError(t, system.SetStoragePolicy(ctx, &policy))

	requestBody := `{"model":"model","messages":[{"role":"user","content":"hello"}]}`
	req := client.Request.Create().
		SetModelID("model").
		SetRequestBody([]byte(requestBody)).
		SetStatus(request.StatusProcessing).
		SaveX(ctx)
	execution := client.RequestExecution.Create().
		SetRequestID(req.ID).
		SetProjectID(req.ProjectID).
		SetModelID("model").
		SetRequestBody([]byte(requestBody)).
		SetStatus(requestexecution.StatusProcessing).
		SaveX(ctx)

	chunks := []*httpclient.StreamEvent{{Type: "message", Data: []byte(`{"delta":"hi"}`)}}
	require.NoError(t, svc.SaveRequestExecutionChunks(ctx, execution.ID, chunks))
	require.NoError(t, svc.SaveRequestChunks(ctx, req.ID, chunks))

	response := map[string]any{"id": "resp-1"}
	require.NoError(t, svc.UpdateRequestExecutionFinalized(
		ctx, execution.ID, requestexecution.StatusCompleted, "", "ext-1", response, nil, "upstream-model",
	))
	require.NoError(t, svc.UpdateRequestFinalized(ctx, req.ID, request.StatusCompleted, "ext-1", response, nil))

	gotExecution := client.RequestExecution.GetX(ctx, execution.ID)
	require.Equal(t, requestexecution.StatusCompleted, gotExecution.Status)
	require.Equal(t, "ext-1", gotExecution.ExternalID)
	require.Equal(t, "upstream-model", gotExecution.UpstreamModelID)
	require.JSONEq(t, requestBody, string(gotExecution.RequestBody))
	require.JSONEq(t, `{"id":"resp-1"}`, string(gotExecution.ResponseBody))
	require.NotEmpty(t, gotExecution.ResponseChunks)

	gotRequest := client.Request.GetX(ctx, req.ID)
	require.Equal(t, request.StatusCompleted, gotRequest.Status)
	require.Equal(t, "ext-1", gotRequest.ExternalID)
	require.JSONEq(t, requestBody, string(gotRequest.RequestBody))
	require.JSONEq(t, `{"id":"resp-1"}`, string(gotRequest.ResponseBody))
	require.NotEmpty(t, gotRequest.ResponseChunks)

	require.NoError(t, svc.UpdateRequestStatus(ctx, req.ID, request.StatusFailed))
	require.Equal(t, request.StatusFailed, client.Request.GetX(ctx, req.ID).Status)
	require.JSONEq(t, requestBody, string(client.Request.GetX(ctx, req.ID).RequestBody))
}
