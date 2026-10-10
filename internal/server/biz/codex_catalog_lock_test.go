package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestCodexCatalog_HTTPRequest_when_WebSocketBase(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "ws", true: "wss"}[secure], func(t *testing.T) {
			// Given an HTTP catalog on a channel configured for WebSocket inference.
			upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/models", r.URL.Path)
				_, _ = w.Write([]byte(`{"models":[]}`))
			}))
			if secure {
				upstream.StartTLS()
			} else {
				upstream.Start()
			}
			defer upstream.Close()
			db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			defer db.Close()
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
			source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetBaseURL("ws" + strings.TrimPrefix(upstream.URL, "http")).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SaveX(ctx)
			svc := NewChannelServiceForTest(db)
			defer svc.Stop()
			built, err := svc.GetChannel(ctx, source.ID)
			require.NoError(t, err)
			built.HTTPClient.GetNativeClient().Transport = upstream.Client().Transport
			svc.SetEnabledChannelsForTest([]*Channel{built})
			// When fetching the native catalog.
			catalog, err := svc.FetchCodexCatalog(ctx, source.ID, "")
			// Then both schemes use HTTP and preserve the catalog.
			require.NoError(t, err)
			require.Zero(t, catalog.Count())
		})
	}
}

func TestCodexCatalog_Error_when_InvalidLock(t *testing.T) {
	// Given a malformed lock entry.
	svc := &ChannelService{}
	svc.codexCatalogLocks.Store(1, "invalid")
	// When fetching a catalog.
	_, err := svc.FetchCodexCatalog(t.Context(), 1, "")
	// Then it returns a typed error instead of panicking.
	var catalogErr *CodexCatalogError
	require.ErrorAs(t, err, &catalogErr)
	require.Equal(t, http.StatusBadGateway, catalogErr.Status)
}

func TestCodexCatalog_Cancelled_when_WaitingForLock(t *testing.T) {
	// Given an occupied channel lock and a cancelled request.
	svc := &ChannelService{}
	lock := semaphore.NewWeighted(1)
	require.NoError(t, lock.Acquire(t.Context(), 1))
	defer lock.Release(1)
	svc.codexCatalogLocks.Store(1, lock)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// When fetching while another request owns the channel lock.
	_, err := svc.FetchCodexCatalog(ctx, 1, "")
	// Then the request exits without waiting or touching the database.
	var catalogErr *CodexCatalogError
	require.ErrorAs(t, err, &catalogErr)
	require.Equal(t, http.StatusBadGateway, catalogErr.Status)
}
