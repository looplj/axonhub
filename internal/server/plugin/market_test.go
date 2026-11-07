package plugin

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
)

func TestMarket(t *testing.T) {
	ctx := pluginCtx()

	t.Run("sha256 mismatch does not insert", func(t *testing.T) {
		zip := marketZip(t, "1.2.0", nil)
		pkgURL := marketPackageURL("guard-1.2.0.axplugin")
		index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
			marketVersionSpec("1.2.0", pkgURL, sha256Hex([]byte("not-the-package")), "^1.0.0"),
		))
		manager, db, hits := newMarket(t, map[string]marketRoute{
			marketIndexPath(t):                        {body: index},
			marketPackagePath("guard-1.2.0.axplugin"): {body: zip},
		})

		_, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "9.9.9", nil, nil)
		require.ErrorContains(t, err, "not available")
		require.Zero(t, countPlugins(t, db))

		_, _, err = manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
		require.ErrorContains(t, err, "sha256")
		require.Equal(t, 1, hits.get(marketPackagePath("guard-1.2.0.axplugin")))
		require.Zero(t, countPlugins(t, db))
	})

	t.Run("matching sha256 matches a direct install", func(t *testing.T) {
		zip := marketZip(t, "1.2.0", nil)
		pkgURL := marketPackageURL("guard-1.2.0.axplugin")
		index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
			marketVersionSpec("1.2.0", pkgURL, sha256Hex(zip), "^1.0.0"),
		))
		manager, db, _ := newMarket(t, map[string]marketRoute{
			marketIndexPath(t):                        {body: index},
			marketPackagePath("guard-1.2.0.axplugin"): {body: zip},
		})
		result, body, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
		require.NoError(t, err)
		require.Nil(t, result.Preview)
		require.Equal(t, zip, body)

		directDB := newPluginDB(t)
		directManager := newTestManager(t, directDB)
		direct, err := directManager.Install(ctx, zip, nil, nil)
		require.NoError(t, err)
		require.Nil(t, direct.Preview)

		fromMarket := onlyPlugin(t, db)
		fromUpload := onlyPlugin(t, directDB)
		require.Equal(t, fromUpload.PluginID, fromMarket.PluginID)
		require.Equal(t, fromUpload.Version, fromMarket.Version)
		require.Equal(t, fromUpload.PackageSha256, fromMarket.PackageSha256)
		require.Equal(t, "acme.failover-guard", fromMarket.PluginID)
		require.Equal(t, "1.2.0", fromMarket.Version)
		require.False(t, fromMarket.Enabled)
		require.False(t, fromUpload.Enabled)
		require.Empty(t, fromMarket.GrantedPermissions)
		require.Empty(t, fromMarket.GrantedNetwork)
		require.Empty(t, fromUpload.GrantedPermissions)
		require.Empty(t, fromUpload.GrantedNetwork)
	})

	t.Run("offers the highest compatible version in index order", func(t *testing.T) {
		sha := sha256Hex([]byte("catalog"))
		compatible := func(version, name string) map[string]any {
			spec := marketVersionSpec(version, marketPackageURL(name), sha, "^1.0.0")
			spec["size"] = 12
			return spec
		}
		index := marketIndex(t,
			map[string]any{"name": "Missing id", "versions": []any{compatible("1.0.0", "missing-id.axplugin")}},
			map[string]any{
				"id":          "acme.failover-guard",
				"name":        "Failover Guard",
				"description": "Notify and disable channels",
				"homepage":    "https://example.com/ignored",
				"versions": []any{
					map[string]any{"version": "0.1.0", "url": marketPackageURL("missing-sha.axplugin")},
					compatible("1.2.0", "guard-1.2.0.axplugin"),
					compatible("1.9.0", "guard-1.9.0.axplugin"),
					compatible("1.10.0", "guard-1.10.0.axplugin"),
					marketVersionSpec("3.0.0", marketPackageURL("guard-3.0.0.axplugin"), sha, ">=9.0.0"),
					marketVersionSpec("1.5.0", "http://downloads.example.com/packages/guard-1.5.0.axplugin", sha, "^1.0.0"),
					marketVersionSpec("nope", marketPackageURL("guard-nope.axplugin"), "ZZ", "^1.0.0"),
				},
			},
			map[string]any{"id": "acme.bad-shape", "name": "Bad", "versions": map[string]any{"version": "1.0.0"}},
			marketPlugin("acme.future", "Future", "Later",
				marketVersionSpec("1.0.0", marketPackageURL("future.axplugin"), sha, ">=9.0.0"),
			),
			marketPlugin("acme.notes", "Notes", "Take notes",
				compatible("0.3.0", "notes.axplugin"),
			),
		)
		manager, _, hits := newMarket(t, map[string]marketRoute{
			marketIndexPath(t): {body: index},
		})
		entries, err := manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		require.Equal(t, "acme.failover-guard", entries[0].ID)
		require.Equal(t, "Failover Guard", entries[0].Name)
		require.Equal(t, "Notify and disable channels", entries[0].Description)
		require.Equal(t, "1.10.0", entries[0].Version)
		require.Equal(t, "^1.0.0", entries[0].AxonhubVersion)
		require.Nil(t, entries[0].InstalledVersion)
		require.Nil(t, entries[0].UpdateFrom)
		require.Equal(t, "acme.notes", entries[1].ID)
		require.Equal(t, "0.3.0", entries[1].Version)
		require.Zero(t, hits.get("dial:http://downloads.example.com/packages/guard-1.5.0.axplugin"))
	})

	t.Run("installs an older compatible version", func(t *testing.T) {
		oldZip := marketZip(t, "1.0.0", nil)
		newZip := marketZip(t, "1.2.0", nil)
		index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
			marketVersionSpec("1.0.0", marketPackageURL("old.axplugin"), sha256Hex(oldZip), "^1.0.0"),
			marketVersionSpec("1.2.0", marketPackageURL("new.axplugin"), sha256Hex(newZip), "^1.0.0"),
		))
		manager, db, hits := newMarket(t, map[string]marketRoute{
			marketIndexPath(t):                {body: index},
			marketPackagePath("old.axplugin"): {body: oldZip},
			marketPackagePath("new.axplugin"): {body: newZip},
		})
		entries, err := manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "1.2.0", entries[0].Version)

		result, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.0.0", nil, nil)
		require.NoError(t, err)
		require.Nil(t, result.Preview)
		require.Equal(t, "1.0.0", result.Plugin.Version)
		require.Equal(t, "1.0.0", onlyPlugin(t, db).Version)
		require.Equal(t, 1, hits.get(marketPackagePath("old.axplugin")))
		require.Zero(t, hits.get(marketPackagePath("new.axplugin")))
	})

	t.Run("updateFrom follows the installed version", func(t *testing.T) {
		index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
			marketVersionSpec("1.2.0", marketPackageURL("guard.axplugin"), sha256Hex([]byte("unused")), "^1.0.0"),
		))
		manager, db, _ := newMarket(t, map[string]marketRoute{
			marketIndexPath(t): {body: index},
		})
		installed := installUI(t, manager, marketZip(t, "1.0.0", nil))

		entries, err := manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "1.2.0", entries[0].Version)
		require.Equal(t, "1.0.0", *entries[0].InstalledVersion)
		require.Equal(t, "1.0.0", *entries[0].UpdateFrom)

		_, err = manager.Install(ctx, marketZip(t, "1.2.0", nil), nil, nil)
		require.NoError(t, err)
		entries, err = manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "1.2.0", *entries[0].InstalledVersion)
		require.Nil(t, entries[0].UpdateFrom)

		_, err = manager.Install(ctx, marketZip(t, "2.0.0", nil), nil, nil)
		require.NoError(t, err)
		entries, err = manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "2.0.0", *entries[0].InstalledVersion)
		require.Nil(t, entries[0].UpdateFrom)

		_, err = db.Plugin.UpdateOneID(installed.ID).SetVersion("not-semver").Save(ctx)
		require.NoError(t, err)
		entries, err = manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "not-semver", *entries[0].InstalledVersion)
		require.Equal(t, "not-semver", *entries[0].UpdateFrom)

		require.NoError(t, manager.Delete(ctx, installed.ID))
		entries, err = manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Nil(t, entries[0].InstalledVersion)
		require.Nil(t, entries[0].UpdateFrom)
	})

	t.Run("redirect and non-https do not write", func(t *testing.T) {
		t.Run("github release asset redirect", func(t *testing.T) {
			zip := marketZip(t, "1.2.0", nil)
			pkgURL := "https://github.com/looplj/axonhub-jev-guard/releases/download/v1.2.0/guard.axplugin"
			assetURL := "https://release-assets.githubusercontent.com/github-production-release-asset/guard.axplugin"
			index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
				marketVersionSpec("1.2.0", pkgURL, sha256Hex(zip), "^1.0.0"),
			))
			manager, db, hits := newMarket(t, map[string]marketRoute{
				marketIndexPath(t): {body: index},
				"/looplj/axonhub-jev-guard/releases/download/v1.2.0/guard.axplugin": {
					redirect: assetURL,
				},
				"/github-production-release-asset/guard.axplugin": {body: zip},
			})
			_, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
			require.NoError(t, err)
			require.Equal(t, 1, hits.get("/looplj/axonhub-jev-guard/releases/download/v1.2.0/guard.axplugin"))
			require.Equal(t, 1, hits.get("/github-production-release-asset/guard.axplugin"))
			require.Equal(t, 1, countPlugins(t, db))
		})

		t.Run("github redirect off github", func(t *testing.T) {
			zip := marketZip(t, "1.2.0", nil)
			pkgURL := "https://github.com/looplj/axonhub-jev-guard/releases/download/v1.2.0/guard.axplugin"
			evilURL := marketPackageURL("evil.axplugin")
			index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
				marketVersionSpec("1.2.0", pkgURL, sha256Hex(zip), "^1.0.0"),
			))
			manager, db, hits := newMarket(t, map[string]marketRoute{
				marketIndexPath(t): {body: index},
				"/looplj/axonhub-jev-guard/releases/download/v1.2.0/guard.axplugin": {
					redirect: evilURL,
				},
				marketPackagePath("evil.axplugin"): {body: zip},
			})
			_, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
			require.ErrorContains(t, err, "redirect")
			require.Equal(t, 1, hits.get("/looplj/axonhub-jev-guard/releases/download/v1.2.0/guard.axplugin"))
			require.Zero(t, hits.get(marketPackagePath("evil.axplugin")))
			require.Zero(t, countPlugins(t, db))
		})

		t.Run("redirect", func(t *testing.T) {
			zip := marketZip(t, "1.2.0", nil)
			pkgURL := marketPackageURL("guard.axplugin")
			evilURL := marketPackageURL("evil.axplugin")
			index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
				marketVersionSpec("1.2.0", pkgURL, sha256Hex(zip), "^1.0.0"),
			))
			manager, db, hits := newMarket(t, map[string]marketRoute{
				marketIndexPath(t):                  {body: index},
				marketPackagePath("guard.axplugin"): {redirect: evilURL},
				marketPackagePath("evil.axplugin"):  {body: zip},
			})
			_, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
			require.ErrorContains(t, err, "redirect")
			require.Equal(t, 1, hits.get(marketPackagePath("guard.axplugin")))
			require.Zero(t, hits.get(marketPackagePath("evil.axplugin")))
			require.Zero(t, countPlugins(t, db))
		})

		t.Run("non-https", func(t *testing.T) {
			httpURL := "http://downloads.example.com/packages/guard.axplugin"
			sha := sha256Hex([]byte("unused"))
			index := marketIndex(t,
				marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
					marketVersionSpec("1.2.0", httpURL, sha, "^1.0.0"),
				),
				marketPlugin("acme.notes", "Notes", "Take notes",
					marketVersionSpec("0.3.0", marketPackageURL("notes.axplugin"), sha, "^1.0.0"),
				),
			)
			manager, db, hits := newMarket(t, map[string]marketRoute{
				marketIndexPath(t): {body: index},
			})
			entries, err := manager.MarketCatalog(ctx)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, "acme.notes", entries[0].ID)

			_, _, err = manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
			require.ErrorContains(t, err, "not available")
			require.Zero(t, hits.get("dial:"+httpURL))
			require.Zero(t, countPlugins(t, db))
		})
	})

	t.Run("catalog fetch failure does not block local install", func(t *testing.T) {
		indexPath := marketIndexPath(t)
		good := marketIndex(t, marketPlugin("acme.notes", "Notes", "Take notes",
			marketVersionSpec("0.3.0", marketPackageURL("notes.axplugin"), sha256Hex([]byte("unused")), "^1.0.0"),
		))
		manager, db, state := newMarket(t, map[string]marketRoute{
			indexPath: {status: http.StatusInternalServerError, body: []byte("unavailable")},
		})
		_, err := manager.MarketCatalog(ctx)
		require.ErrorContains(t, err, "status 500")
		require.Equal(t, 1, state.get(indexPath))

		installed, err := manager.Install(ctx, marketZip(t, "1.0.0", nil), nil, nil)
		require.NoError(t, err)
		require.Nil(t, installed.Preview)
		require.Equal(t, "1.0.0", onlyPlugin(t, db).Version)

		state.set(indexPath, marketRoute{body: good})
		entries, err := manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "acme.notes", entries[0].ID)
		require.Equal(t, 2, state.get(indexPath))
	})

	t.Run("caches a successful index for five minutes", func(t *testing.T) {
		indexPath := marketIndexPath(t)
		index := marketIndex(t, marketPlugin("acme.notes", "Notes", "Take notes",
			marketVersionSpec("0.3.0", marketPackageURL("notes.axplugin"), sha256Hex([]byte("unused")), "^1.0.0"),
		))
		manager, _, state := newMarket(t, map[string]marketRoute{
			indexPath: {body: index},
		})
		start := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
		now := start
		manager.SetMarketClock(func() time.Time { return now })

		entries, err := manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "acme.notes", entries[0].ID)
		require.Equal(t, 1, state.get(indexPath))

		now = start.Add(marketIndexTTL - time.Second)
		state.set(indexPath, marketRoute{status: http.StatusInternalServerError})
		entries, err = manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "acme.notes", entries[0].ID)
		require.Equal(t, 1, state.get(indexPath))

		now = start.Add(marketIndexTTL)
		_, err = manager.MarketCatalog(ctx)
		require.ErrorContains(t, err, "status 500")
		require.Equal(t, 2, state.get(indexPath))

		state.set(indexPath, marketRoute{body: index})
		entries, err = manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "acme.notes", entries[0].ID)
		require.Equal(t, 3, state.get(indexPath))
	})

	t.Run("rejects a malformed index", func(t *testing.T) {
		cases := []struct {
			name string
			body string
			want string
		}{
			{name: "schema version", body: `{"schemaVersion":2,"plugins":[]}`, want: "schemaVersion"},
			{name: "plugins object", body: `{"schemaVersion":1,"plugins":{}}`, want: "plugins"},
			{name: "invalid json", body: `{`, want: "JSON"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				manager, _, _ := newMarket(t, map[string]marketRoute{
					marketIndexPath(t): {body: []byte(tc.body)},
				})
				_, err := manager.MarketCatalog(ctx)
				require.ErrorContains(t, err, tc.want)
			})
		}
	})

	t.Run("does not cache a malformed index", func(t *testing.T) {
		indexPath := marketIndexPath(t)
		good := marketIndex(t, marketPlugin("acme.notes", "Notes", "Take notes",
			marketVersionSpec("0.3.0", marketPackageURL("notes.axplugin"), sha256Hex([]byte("unused")), "^1.0.0"),
		))
		manager, _, state := newMarket(t, map[string]marketRoute{
			indexPath: {body: []byte(`{"schemaVersion":2,"plugins":[]}`)},
		})
		_, err := manager.MarketCatalog(ctx)
		require.ErrorContains(t, err, "schemaVersion")
		require.Equal(t, 1, state.get(indexPath))

		state.set(indexPath, marketRoute{body: good})
		entries, err := manager.MarketCatalog(ctx)
		require.NoError(t, err)
		require.Equal(t, "acme.notes", entries[0].ID)
		require.Equal(t, 2, state.get(indexPath))
	})

	t.Run("manifest identity mismatch does not write", func(t *testing.T) {
		zip := marketZip(t, "1.0.0", nil)
		sha := sha256Hex(zip)
		index := marketIndex(t,
			marketPlugin("acme.notes", "Notes", "Take notes",
				marketVersionSpec("1.0.0", marketPackageURL("notes.axplugin"), sha, "^1.0.0"),
			),
			marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
				marketVersionSpec("1.2.0", marketPackageURL("guard.axplugin"), sha, "^1.0.0"),
			),
		)
		manager, db, _ := newMarket(t, map[string]marketRoute{
			marketIndexPath(t):                  {body: index},
			marketPackagePath("notes.axplugin"): {body: zip},
			marketPackagePath("guard.axplugin"): {body: zip},
		})

		_, _, err := manager.InstallFromMarket(ctx, "acme.notes", "1.0.0", nil, nil)
		require.ErrorContains(t, err, "manifest does not match")
		require.Zero(t, countPlugins(t, db))

		_, _, err = manager.InstallFromMarket(ctx, "acme.failover-guard", "1.2.0", nil, nil)
		require.ErrorContains(t, err, "manifest does not match")
		require.Zero(t, countPlugins(t, db))
	})

	t.Run("enabled upgrade asks before writing a new permission", func(t *testing.T) {
		oldZip := marketZip(t, "1.0.0", nil)
		newZip := marketZip(t, "1.1.0", []any{"read_channels", "webhook", "write_channels"})
		pkgPath := marketPackagePath("guard-1.1.0.axplugin")
		index := marketIndex(t, marketPlugin("acme.failover-guard", "Failover Guard", "Notify",
			marketVersionSpec("1.1.0", marketPackageURL("guard-1.1.0.axplugin"), sha256Hex(newZip), "^1.0.0"),
		))
		manager, db, hits := newMarket(t, map[string]marketRoute{
			marketIndexPath(t): {body: index},
			pkgPath:            {body: newZip},
		})
		installed := installUI(t, manager, oldZip)
		require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels", "webhook"}, nil))

		preview, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.1.0", nil, nil)
		require.NoError(t, err)
		require.NotNil(t, preview.Preview)
		require.Equal(t, "1.0.0", preview.Preview.CurrentVersion)
		require.Equal(t, "1.1.0", preview.Preview.NewVersion)
		require.Equal(t, []string{"write_channels"}, preview.Preview.Permissions)
		require.Equal(t, "1.0.0", onlyPlugin(t, db).Version)
		require.NotContains(t, onlyPlugin(t, db).Manifest.Permissions, "write_channels")
		require.Equal(t, 1, hits.get(pkgPath))

		grant := []string{"read_channels", "webhook", "write_channels"}
		upgraded, _, err := manager.InstallFromMarket(ctx, "acme.failover-guard", "1.1.0", &grant, nil)
		require.NoError(t, err)
		require.Nil(t, upgraded.Preview)
		require.True(t, upgraded.Plugin.Enabled)
		require.Equal(t, "1.1.0", upgraded.Plugin.Version)
		require.Equal(t, grant, upgraded.Plugin.GrantedPermissions)
		require.Equal(t, "1.1.0", onlyPlugin(t, db).Version)
		require.Equal(t, 2, hits.get(pkgPath))
	})
}

type marketRoute struct {
	status   int
	body     []byte
	redirect string
}

type marketHTTP struct {
	mu     sync.Mutex
	routes map[string]marketRoute
	hits   map[string]int
}

func (m *marketHTTP) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.hits[r.URL.Path]++
	route, ok := m.routes[r.URL.Path]
	m.mu.Unlock()
	if !ok {
		http.Error(w, "no market route for "+r.URL.Path, http.StatusNotFound)
		return
	}
	if route.redirect != "" {
		http.Redirect(w, r, route.redirect, http.StatusFound)
		return
	}
	status := route.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(route.body)
}

func (m *marketHTTP) get(path string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hits[path]
}

func (m *marketHTTP) set(path string, route marketRoute) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[path] = route
}

type marketRoundTrip func(*http.Request) (*http.Response, error)

func (f marketRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMarket(t *testing.T, routes map[string]marketRoute) (*Manager, *ent.Client, *marketHTTP) {
	t.Helper()
	state := &marketHTTP{
		routes: map[string]marketRoute{},
		hits:   map[string]int{},
	}
	maps.Copy(state.routes, routes)
	server := httptest.NewTLSServer(http.HandlerFunc(state.serve))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	base := server.Client().Transport
	client := &http.Client{
		Transport: marketRoundTrip(func(req *http.Request) (*http.Response, error) {
			if req.URL.Scheme != "https" {
				state.mu.Lock()
				state.hits["dial:"+req.URL.String()]++
				state.mu.Unlock()
				return nil, fmt.Errorf("dialed non-https url %s", req.URL)
			}
			clone := req.Clone(req.Context())
			clone.URL.Scheme = target.Scheme
			clone.URL.Host = target.Host
			clone.Host = target.Host
			return base.RoundTrip(clone)
		}),
	}

	db := newPluginDB(t)
	manager := newTestManager(t, db)
	manager.SetMarketClient(client)
	frozen := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	manager.SetMarketClock(func() time.Time { return frozen })
	return manager, db, state
}

func marketIndexPath(t *testing.T) string {
	t.Helper()
	parsed, err := url.Parse(MarketIndexURL)
	require.NoError(t, err)
	return parsed.Path
}

func marketPackageURL(name string) string {
	return "https://downloads.example.com" + marketPackagePath(name)
}

func marketPackagePath(name string) string {
	return "/packages/" + name
}

func marketIndex(t *testing.T, plugins ...any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"generatedAt":   "ignored-by-this-hub",
		"plugins":       plugins,
	})
	require.NoError(t, err)
	return body
}

func marketPlugin(id, name, description string, versions ...any) map[string]any {
	return map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
		"versions":    versions,
	}
}

func marketVersionSpec(version, rawURL, sha, axon string) map[string]any {
	return map[string]any{
		"version":        version,
		"url":            rawURL,
		"sha256":         sha,
		"axonhubVersion": axon,
	}
}

func marketZip(t *testing.T, version string, permissions []any) []byte {
	t.Helper()
	return uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = version
		if permissions != nil {
			manifest["permissions"] = permissions
		}
	}, map[string]string{"ui/page.html": "<html>" + version + "</html>"})
}

func countPlugins(t *testing.T, client *ent.Client) int {
	t.Helper()
	n, err := client.Plugin.Query().Count(pluginCtx())
	require.NoError(t, err)
	return n
}

func onlyPlugin(t *testing.T, client *ent.Client) *ent.Plugin {
	t.Helper()
	rows, err := client.Plugin.Query().All(pluginCtx())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	return rows[0]
}
