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
)

func TestUpdateResponseHeadersMasksSensitiveValues(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:response-headers?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	system := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	storage := NewDataStorageService(DataStorageServiceParams{Client: client, SystemService: system, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	svc := NewRequestService(client, system.CacheConfig, system, nil, storage, NewLiveStreamRegistry())
	req, err := client.Request.Create().SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(request.StatusProcessing).Save(ctx)
	require.NoError(t, err)
	execution, err := client.RequestExecution.Create().SetRequestID(req.ID).SetProjectID(req.ProjectID).SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing).Save(ctx)
	require.NoError(t, err)

	req, err = client.Request.Get(ctx, execution.RequestID)
	require.NoError(t, err)
	headers := http.Header{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer secret"},
		"X-Request-ID":  {"request-id"},
	}

	require.NoError(t, svc.UpdateRequestResponseHeaders(ctx, req.ID, headers))
	require.NoError(t, svc.UpdateRequestExecutionResponseHeaders(ctx, execution.ID, headers))

	req = client.Request.GetX(ctx, req.ID)
	require.JSONEq(t, `{"Content-Type":["application/json"],"Authorization":["******"],"X-Request-ID":["request-id"]}`, string(req.ResponseHeaders))

	execution = client.RequestExecution.GetX(ctx, execution.ID)
	require.JSONEq(t, `{"Content-Type":["application/json"],"Authorization":["******"],"X-Request-ID":["request-id"]}`, string(execution.ResponseHeaders))
}
