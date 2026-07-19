//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/server/middleware"
)

func TestReloadIPAccessControl(t *testing.T) {
	loader, runtime, configFile := newIPAccessControlReloadFixture(t, "192.0.2.1")

	writeReloadConfig(t, configFile, "203.0.113.0/24")
	if err := reloadIPAccessControl(context.Background(), loader, runtime); err != nil {
		t.Fatalf("reloadIPAccessControl() error = %v", err)
	}

	if ipAccessAllowed(t, runtime, "192.0.2.1") {
		t.Fatal("old IP should not remain allowed after a successful reload")
	}
	if !ipAccessAllowed(t, runtime, "203.0.113.10") {
		t.Fatal("new CIDR should be allowed after a successful reload")
	}
}

func TestReloadIPAccessControlKeepsPreviousStateOnInvalidConfig(t *testing.T) {
	loader, runtime, configFile := newIPAccessControlReloadFixture(t, "192.0.2.1")

	writeReloadConfig(t, configFile, "not-an-ip")
	if err := reloadIPAccessControl(context.Background(), loader, runtime); err == nil {
		t.Fatal("reloadIPAccessControl() error = nil, want invalid IP error")
	}

	if !ipAccessAllowed(t, runtime, "192.0.2.1") {
		t.Fatal("invalid reload must leave the previous IP rules active")
	}
}

func newIPAccessControlReloadFixture(t *testing.T, allowedIP string) (*conf.Loader, *middleware.IPAccessControlConfig, string) {
	t.Helper()

	configDir := t.TempDir()
	t.Chdir(configDir)

	configFile := filepath.Join(configDir, "config.yml")
	writeReloadConfig(t, configFile, allowedIP)

	initial, loader, err := conf.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	runtime, err := middleware.NewIPAccessControlConfig(
		initial.APIServer.IPAccessControl.Enabled,
		initial.APIServer.IPAccessControl.AllowedIPs,
		initial.APIServer.IPAccessControl.RedirectURL,
	)
	if err != nil {
		t.Fatalf("NewIPAccessControlConfig() error = %v", err)
	}

	return loader, runtime, configFile
}

func writeReloadConfig(t *testing.T, configFile string, allowedIP string) {
	t.Helper()

	contents := "server:\n  ip_access_control:\n    enabled: true\n    allowed_ips:\n      - " + allowedIP + "\n"
	if err := os.WriteFile(configFile, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func ipAccessAllowed(t *testing.T, config *middleware.IPAccessControlConfig, clientIP string) bool {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = clientIP + ":12345"
	ctx.Request = request

	middleware.WithIPAccessControl(config)(ctx)
	return recorder.Code == http.StatusOK
}
