package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

func TestOAuthProvider_ReconcilesDatabaseThenPersistsNextRefresh(t *testing.T) {
	for _, detached := range []bool{false, true} {
		for _, scenario := range []string{"oauth", "legacy", "api-key", "metadata", "expired"} {
			t.Run(scenario+map[bool]string{false: "/Get", true: "/EnsureFresh"}[detached], func(t *testing.T) {
				// Given: a provider with an old snapshot and a newer database row.
				db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
				defer db.Close()
				ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
				initial := &oauth.OAuthCredentials{AccessToken: "source", RefreshToken: "source-refresh", ExpiresAt: time.Now().Add(-time.Hour)}
				source := db.Channel.Create().SetName("oauth").SetType(channel.TypeCodex).SetBaseURL("https://example.com").SetDefaultTestModel("test").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{OAuth: initial}).SaveX(ctx)
				svc := NewChannelServiceForTest(db)
				defer svc.Stop()
				callback := svc.onTokenRefreshed(source)
				if detached {
					callback = svc.onTokenRefreshedDetached(source)
				}
				winnerExpiry := 30 * time.Minute
				if scenario == "expired" {
					winnerExpiry = -time.Hour
				}
				winner := &oauth.OAuthCredentials{AccessToken: "winner", RefreshToken: "winner-refresh", ExpiresAt: time.Now().Add(winnerExpiry)}
				update := db.Channel.UpdateOneID(source.ID).SetName("admin-name")
				switch scenario {
				case "oauth":
					update.SetCredentials(objects.ChannelCredentials{OAuth: winner})
				case "legacy":
					encoded, err := winner.ToJSON()
					require.NoError(t, err)
					update.SetCredentials(objects.ChannelCredentials{APIKey: encoded})
				case "api-key":
					update.SetCredentials(objects.ChannelCredentials{APIKey: "admin-key"})
				case "expired":
					update.SetCredentials(objects.ChannelCredentials{OAuth: winner})
				case "metadata":
				}
				update.SaveX(ctx)
				var calls atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if err := r.ParseForm(); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					switch r.Form.Get("refresh_token") {
					case "source-refresh":
						_, _ = w.Write([]byte(`{"access_token":"rotated","refresh_token":"rotated-refresh","expires_in":3600}`))
					case "winner-refresh", "rotated-refresh":
						_, _ = w.Write([]byte(`{"access_token":"next","refresh_token":"next-refresh","expires_in":3600}`))
					default:
						t.Error("provider used a losing refresh chain")
						w.WriteHeader(http.StatusUnauthorized)
					}
				}))
				defer upstream.Close()
				provider := oauth.NewTokenProvider(oauth.TokenProviderParams{
					Credentials: initial, HTTPClient: httpclient.NewHttpClientWithClient(upstream.Client()),
					OAuthUrls: oauth.OAuthUrls{TokenUrl: upstream.URL}, OnRefreshed: callback,
				})

				// When: refresh through the real provider and persistence callback.
				var got *oauth.OAuthCredentials
				var err error
				if detached {
					got, err = provider.EnsureFresh(ctx, time.Minute)
				} else {
					got, err = provider.Get(ctx)
				}

				// Then: the provider follows the database winner, never the discarded chain.
				if scenario == "api-key" {
					require.Error(t, err)
					require.Nil(t, got)
					_, err = provider.Get(ctx)
					require.Error(t, err)
					require.Equal(t, int32(1), calls.Load())
					require.Equal(t, "admin-key", db.Channel.GetX(ctx, source.ID).Credentials.APIKey)
					return
				}
				require.NoError(t, err)
				want := "winner"
				if scenario == "metadata" {
					want = "rotated"
				} else if scenario == "expired" {
					want = "next"
				}
				require.Equal(t, want, got.AccessToken)
				stored, err := db.Channel.GetX(ctx, source.ID).Credentials.ResolveOAuthCredentials()
				require.NoError(t, err)
				require.Equal(t, got.RefreshToken, stored.RefreshToken)

				// When: the same provider refreshes again without resetting its credentials.
				followUpRefreshBefore := 2 * time.Hour
				if scenario == "expired" {
					followUpRefreshBefore = time.Minute
				}
				got, err = provider.EnsureFresh(context.WithoutCancel(ctx), followUpRefreshBefore)
				// Then: the next rotation is persisted and the shared source stays immutable.
				require.NoError(t, err)
				require.Equal(t, "next-refresh", got.RefreshToken)
				saved := db.Channel.GetX(ctx, source.ID)
				require.Equal(t, "next-refresh", saved.Credentials.OAuth.RefreshToken)
				require.Equal(t, "admin-name", saved.Name)
				require.Equal(t, "source", source.Credentials.OAuth.AccessToken)
			})
		}
	}
}

func TestChannelCredentialsEqualUsesOAuthInstant(t *testing.T) {
	location := time.FixedZone("test", 9*60*60)
	instant := time.Date(2026, 10, 11, 12, 0, 0, 0, location)

	left := objects.ChannelCredentials{OAuth: &oauth.OAuthCredentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    instant,
	}}
	right := objects.ChannelCredentials{OAuth: &oauth.OAuthCredentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    instant.UTC(),
	}}

	require.True(t, channelCredentialsEqual(left, right))
}
