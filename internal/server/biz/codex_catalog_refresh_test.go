package biz

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/stretchr/testify/require"
)

func TestCodexCatalog_ConcurrentRefresh(t *testing.T) {
	var refreshes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			refreshes.Add(1)
			_, _ = w.Write([]byte(`{"access_token":"renewed","refresh_token":"rotated","expires_in":3600,"token_type":"bearer"}`))
			return
		}
		require.Equal(t, "Bearer renewed", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	original := codex.DefaultTokenURLs
	codex.DefaultTokenURLs.TokenUrl = upstream.URL + "/token"
	defer func() { codex.DefaultTokenURLs = original }()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("oauth").SetType(channel.TypeCodex).SetStatus(channel.StatusDisabled).SetBaseURL(upstream.URL).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "expired", RefreshToken: "old", ExpiresAt: time.Now().Add(-time.Hour)}}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("catalog worker panic: %v", p)
				}
			}()
			_, err := svc.FetchCodexCatalog(ctx, source.ID, "")
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), refreshes.Load())
	stored := db.Channel.GetX(ctx, source.ID)
	require.Equal(t, "rotated", stored.Credentials.OAuth.RefreshToken)
	require.Equal(t, channel.StatusDisabled, stored.Status)
}

func TestCodexCatalog_CacheTransition(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer source-key", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetName("catalog").SetType(channel.TypeCodex).SetBaseURL(upstream.URL).SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "source-key"}).SaveX(ctx)
	svc := NewChannelServiceForTest(db)
	defer svc.Stop()
	built, err := svc.GetChannel(ctx, source.ID)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{built})
	_, err = svc.FetchCodexCatalog(ctx, source.ID, "")
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{})
	db.Channel.UpdateOneID(source.ID).SetStatus(channel.StatusArchived).SaveX(ctx)
	_, err = svc.FetchCodexCatalog(ctx, source.ID, "")
	require.NoError(t, err)
	require.Equal(t, channel.StatusArchived, db.Channel.GetX(ctx, source.ID).Status)
}
