package biz

import (
	"context"
	"net/http"
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

// Finalize writers persist response metadata and chunks together while leaving
// the original request payload intact.
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

	// The channel is assigned in memory at selection and persisted by the terminal update.
	svc.AssignRequestChannel(ctx, req, 7)
	require.Zero(t, client.Request.GetX(ctx, req.ID).ChannelID)

	response := map[string]any{"id": "resp-1"}
	headers := http.Header{"Content-Type": {"application/json"}, "Authorization": {"Bearer secret"}}
	require.NoError(t, svc.UpdateRequestExecutionFinalized(
		ctx, execution, requestexecution.StatusCompleted, "", "ext-1", response, nil, "upstream-model", headers, chunks...,
	))
	require.NoError(t, svc.UpdateRequestFinalized(ctx, req, request.StatusCompleted, "ext-1", response, nil, chunks...))

	gotExecution := client.RequestExecution.GetX(ctx, execution.ID)
	require.Equal(t, requestexecution.StatusCompleted, gotExecution.Status)
	require.Equal(t, "ext-1", gotExecution.ExternalID)
	require.Equal(t, "upstream-model", gotExecution.UpstreamModelID)
	require.JSONEq(t, requestBody, string(gotExecution.RequestBody))
	require.JSONEq(t, `{"id":"resp-1"}`, string(gotExecution.ResponseBody))
	require.NotEmpty(t, gotExecution.ResponseChunks)
	require.JSONEq(t, `{"Content-Type":["application/json"],"Authorization":["******"]}`, string(gotExecution.ResponseHeaders))

	gotRequest := client.Request.GetX(ctx, req.ID)
	require.Equal(t, request.StatusCompleted, gotRequest.Status)
	require.Equal(t, "ext-1", gotRequest.ExternalID)
	require.JSONEq(t, requestBody, string(gotRequest.RequestBody))
	require.JSONEq(t, `{"id":"resp-1"}`, string(gotRequest.ResponseBody))
	require.NotEmpty(t, gotRequest.ResponseChunks)
	require.Equal(t, 7, gotRequest.ChannelID)

	require.NoError(t, svc.UpdateRequestStatus(ctx, req.ID, request.StatusFailed))
	require.Equal(t, request.StatusFailed, client.Request.GetX(ctx, req.ID).Status)
	require.JSONEq(t, requestBody, string(client.Request.GetX(ctx, req.ID).RequestBody))
}
