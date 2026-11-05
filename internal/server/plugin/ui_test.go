package plugin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPluginUI(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	const (
		secret    = "test-plugin-ui-secret"
		pluginID  = "acme.failover-guard"
		entry     = "ui/overview/index.html"
		htmlBody  = "<!doctype html><html><body>overview</body></html>"
		jsBody    = "console.log('app')"
		cssBody   = "body{color:red}"
		svgBody   = `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
		pngBody   = "png-bytes"
		woffBody  = "woff2-bytes"
		otherHTML = "<html>other-entry</html>"
		txtBody   = "SECRET_TXT_CONTENT"
	)
	installed := installUI(t, manager, uiZip(t, func(t *testing.T, manifest map[string]any) {
		page0(t, manifest)["entry"] = entry
	}, map[string]string{
		entry:                   htmlBody,
		"ui/overview/app.js":    jsBody,
		"ui/overview/style.css": cssBody,
		"ui/assets/logo.svg":    svgBody,
		"ui/assets/pixel.png":   pngBody,
		"ui/assets/font.woff2":  woffBody,
		"ui/other.html":         otherHTML,
		"ui/readme.txt":         txtBody,
	}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil))
	require.Equal(t, RuntimeStatusActive, manager.Current().ByPluginID(pluginID).Status)

	bridge, err := os.ReadFile("uisdk/bridge.js")
	require.NoError(t, err)
	now := time.Now()
	token, err := manager.IssueUIToken(secret, pluginID, entry, 7, now)
	require.NoError(t, err)
	_, err = manager.IssueUIToken(secret, pluginID, entry, 0, now)
	require.Error(t, err)
	_, err = manager.IssueUIToken(secret, pluginID, "ui/other.html", 7, now)
	require.Error(t, err)

	request := func(method, rawToken, resourcePath, origin string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, "http://example.test/admin/plugin-ui/resource", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		ServePluginUI(recorder, req, PluginUIRequest{
			Manager: manager,
			Secret:  secret,
			Token:   rawToken,
			Path:    resourcePath,
		})
		return recorder
	}

	t.Run("bound resources and bridge", func(t *testing.T) {
		cases := []struct {
			path string
			mime string
			body string
		}{
			{path: "/" + entry, mime: "text/html", body: htmlBody},
			{path: "/ui/overview/app.js", mime: "text/javascript", body: jsBody},
			{path: "/ui/overview/style.css", mime: "text/css", body: cssBody},
			{path: "/ui/assets/logo.svg", mime: "image/svg+xml", body: svgBody},
			{path: "/ui/assets/pixel.png", mime: "image/png", body: pngBody},
			{path: "/ui/assets/font.woff2", mime: "font/woff2", body: woffBody},
			{path: "/_axonhub/bridge.js", mime: "text/javascript", body: string(bridge)},
		}
		for _, tc := range cases {
			response := request(http.MethodGet, token, tc.path, "null")
			require.Equal(t, http.StatusOK, response.Code, tc.path)
			require.Equal(t, tc.mime, response.Header().Get("Content-Type"), tc.path)
			require.Equal(t, tc.body, response.Body.String(), tc.path)
			assertPluginUIHeaders(t, response.Header())
		}
	})

	t.Run("head", func(t *testing.T) {
		response := request(http.MethodHead, token, "/"+entry, "null")
		require.Equal(t, http.StatusOK, response.Code)
		require.Empty(t, response.Body.String())
		require.Equal(t, "text/html", response.Header().Get("Content-Type"))
		assertPluginUIHeaders(t, response.Header())
	})

	t.Run("rejected tokens keep anonymous cors", func(t *testing.T) {
		expired, err := manager.IssueUIToken(secret, pluginID, entry, 7, now.Add(-2*time.Hour))
		require.NoError(t, err)
		payload, signature, ok := strings.Cut(token, ".")
		require.True(t, ok)
		replacement := "A"
		if signature[0] == 'A' {
			replacement = "B"
		}
		tampered := payload + "." + replacement + signature[1:]
		for _, rawToken := range []string{"not-a-token", expired, tampered, token + ".extra"} {
			recorder := httptest.NewRecorder()
			recorder.Header().Set("Vary", "Accept-Encoding")
			recorder.Header().Set("Access-Control-Allow-Origin", "null")
			recorder.Header().Set("Access-Control-Allow-Credentials", "true")
			req := httptest.NewRequest(http.MethodGet, "http://example.test/admin/plugin-ui/"+rawToken+"/"+entry, nil)
			req.Header.Set("Origin", "null")
			ServePluginUI(recorder, req, PluginUIRequest{
				Manager: manager,
				Secret:  secret,
				Token:   rawToken,
				Path:    "/" + entry,
			})
			require.Equal(t, http.StatusForbidden, recorder.Code, rawToken)
			require.NotContains(t, recorder.Body.String(), "overview")
			assertPluginUIHeaders(t, recorder.Header())
			require.Equal(t, "Accept-Encoding", recorder.Header().Get("Vary"))
		}
	})

	t.Run("out of scope paths", func(t *testing.T) {
		paths := []string{
			"/../secret.txt",
			"/ui/../../secret.txt",
			"/ui/overview/../../../etc/passwd",
			"/%2e%2e/secret.txt",
			"/ui/%2e%2e/app.js",
			`/ui\app.js`,
			"/%5Csecret.txt",
			"//etc/passwd",
			"/",
			"/manifest.json",
			"/plugin.js",
			"/ui/other.html",
			"/ui/readme.txt",
			"/ui/page.HTML",
		}
		for _, resourcePath := range paths {
			response := request(http.MethodGet, token, resourcePath, "null")
			require.Equal(t, http.StatusForbidden, response.Code, resourcePath)
			body := response.Body.String()
			require.NotContains(t, body, "overview")
			require.NotContains(t, body, "other-entry")
			require.NotContains(t, body, txtBody)
			require.NotContains(t, body, "schemaVersion")
			require.NotContains(t, strings.ToLower(response.Header().Get("Content-Type")), "html")
			assertPluginUIHeaders(t, response.Header())
		}
	})

	t.Run("missing whitelist file", func(t *testing.T) {
		response := request(http.MethodGet, token, "/ui/overview/missing.js", "null")
		require.Equal(t, http.StatusNotFound, response.Code)
		require.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
		require.NotContains(t, response.Body.String(), "<")
		require.NotContains(t, response.Body.String(), "overview")
		assertPluginUIHeaders(t, response.Header())
	})

	t.Run("other methods", func(t *testing.T) {
		response := request(http.MethodPost, token, "/"+entry, "null")
		require.Equal(t, http.StatusMethodNotAllowed, response.Code)
		require.NotContains(t, response.Body.String(), "overview")
		assertPluginUIHeaders(t, response.Header())
	})

	t.Run("nil manager", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.test/admin/plugin-ui/x/ui/overview/app.js", nil)
		ServePluginUI(recorder, req, PluginUIRequest{Secret: secret, Token: token, Path: "/ui/overview/app.js"})
		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		assertPluginUIHeaders(t, recorder.Header())
	})

	t.Run("disabled plugin", func(t *testing.T) {
		require.NoError(t, manager.Disable(ctx, installed.ID))
		t.Cleanup(func() {
			require.NoError(t, manager.Enable(ctx, installed.ID, nil))
		})
		response := request(http.MethodGet, token, "/"+entry, "null")
		require.Equal(t, http.StatusForbidden, response.Code)
		require.NotContains(t, response.Body.String(), "overview")
		assertPluginUIHeaders(t, response.Header())
	})

	t.Run("deleted directory asks for rebuild", func(t *testing.T) {
		dir := manager.Current().ByPluginID(pluginID).ResourceDir()
		require.NotEmpty(t, dir)
		require.NoError(t, os.RemoveAll(dir))
		response := request(http.MethodGet, token, "/"+entry, "null")
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.NotContains(t, response.Body.String(), "overview")
		assertPluginUIHeaders(t, response.Header())

		require.NoError(t, manager.Refresh(ctx))
		restored := request(http.MethodGet, token, "/"+entry, "null")
		require.Equal(t, http.StatusOK, restored.Code)
		require.Equal(t, htmlBody, restored.Body.String())
		assertPluginUIHeaders(t, restored.Header())
	})

	t.Run("credentialed global cors does not stick", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		config := cors.DefaultConfig()
		config.AllowOrigins = []string{"https://admin.example"}
		config.AllowCredentials = true
		engine.Use(cors.New(config))
		engine.Any("/admin/plugin-ui/:token/*path", func(c *gin.Context) {
			ServePluginUI(c.Writer, c.Request, PluginUIRequest{
				Manager: manager,
				Secret:  secret,
				Token:   c.Param("token"),
				Path:    c.Param("path"),
			})
		})

		allowed := httptest.NewRequest(http.MethodGet, "/admin/plugin-ui/"+token+"/ui/overview/app.js", nil)
		allowed.Header.Set("Origin", "https://admin.example")
		allowedResponse := httptest.NewRecorder()
		engine.ServeHTTP(allowedResponse, allowed)
		require.Equal(t, http.StatusOK, allowedResponse.Code)
		require.Equal(t, jsBody, allowedResponse.Body.String())
		assertPluginUIHeaders(t, allowedResponse.Header())
		require.NotEqual(t, "https://admin.example", allowedResponse.Header().Get("Access-Control-Allow-Origin"))

		denied := httptest.NewRequest(http.MethodGet, "/admin/plugin-ui/not-a-token/ui/overview/app.js", nil)
		denied.Header.Set("Origin", "https://admin.example")
		deniedResponse := httptest.NewRecorder()
		engine.ServeHTTP(deniedResponse, denied)
		require.Equal(t, http.StatusForbidden, deniedResponse.Code)
		require.NotContains(t, deniedResponse.Body.String(), jsBody)
		assertPluginUIHeaders(t, deniedResponse.Header())
	})

	t.Run("upgrade rejects the previous token", func(t *testing.T) {
		upgraded := installUI(t, manager, uiZip(t, func(t *testing.T, manifest map[string]any) {
			manifest["version"] = "1.2.0"
			page0(t, manifest)["entry"] = entry
		}, map[string]string{
			entry:                   "<!doctype html><html><body>upgraded</body></html>",
			"ui/overview/app.js":    jsBody,
			"ui/overview/style.css": cssBody,
			"ui/assets/logo.svg":    svgBody,
			"ui/assets/pixel.png":   pngBody,
			"ui/assets/font.woff2":  woffBody,
		}))
		require.Equal(t, installed.ID, upgraded.ID)
		response := request(http.MethodGet, token, "/"+entry, "null")
		require.Equal(t, http.StatusForbidden, response.Code)
		require.NotContains(t, response.Body.String(), "upgraded")
		require.NotContains(t, response.Body.String(), "overview")
		assertPluginUIHeaders(t, response.Header())

		next, err := manager.IssueUIToken(secret, pluginID, entry, 7, time.Now())
		require.NoError(t, err)
		restored := request(http.MethodGet, next, "/"+entry, "null")
		require.Equal(t, http.StatusOK, restored.Code)
		require.Equal(t, "<!doctype html><html><body>upgraded</body></html>", restored.Body.String())
		assertPluginUIHeaders(t, restored.Header())
	})
}

func assertPluginUIHeaders(t *testing.T, header http.Header) {
	t.Helper()
	require.Equal(t, pluginUICSP, header.Get("Content-Security-Policy"))
	require.Equal(t, "nosniff", header.Get("X-Content-Type-Options"))
	require.Equal(t, "no-referrer", header.Get("Referrer-Policy"))
	require.Equal(t, "*", header.Get("Access-Control-Allow-Origin"))
	_, exists := header["Access-Control-Allow-Credentials"]
	require.False(t, exists)
}
