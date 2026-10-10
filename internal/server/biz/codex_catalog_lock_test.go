package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

func TestCodexCatalog_Cancelled_whenWaitingForFetch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			_, _ = w.Write([]byte(`{"models":[]}`))
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetBaseURL(upstream.URL).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()
	built, err := svc.GetChannel(ctx, source.ID)
	require.NoError(t, err)
	built.HTTPClient.GetNativeClient().Transport = upstream.Client().Transport
	svc.SetEnabledChannelsForTest([]*Channel{built})
	firstDone := make(chan error, 1)
	go func() {
		_, firstErr := svc.FetchCodexCatalog(ctx, source.ID, "")
		firstDone <- firstErr
	}()
	<-started

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = svc.FetchCodexCatalog(ctx, source.ID, "")
	var catalogErr *CodexCatalogError
	require.ErrorAs(t, err, &catalogErr)
	require.Equal(t, http.StatusBadGateway, catalogErr.Status)
	close(release)
	require.NoError(t, <-firstDone)
}
