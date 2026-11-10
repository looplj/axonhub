package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestDesktopLoginFirstRedeemSetsSession(t *testing.T) {
	engine := newDesktopLoginEngine(t)
	const (
		token     = `tok"en`
		projectID = "gid://axonhub/Project/4"
		nextPath  = "/settings/models"
	)

	path := mintDesktopLogin(t, token, projectID, nextPath)
	assertHandoffPath(t, path, token, projectID)

	rec := redeemDesktopLogin(engine, path, "127.0.0.1:12345")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q", got)
	}
	body := rec.Body.String()
	tokenJS := mustJSONString(t, token)
	projectJS := mustJSONString(t, projectID)
	nextJS := mustJSONString(t, nextPath)
	for _, want := range []string{
		`localStorage.setItem("axonhub_access_token", ` + tokenJS + `);`,
		`localStorage.setItem("axonhub_selected_project_id", ` + projectJS + `);`,
		`location.replace(` + nextJS + `);`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %s\n%s", want, body)
		}
	}
	if code := handoffCode(t, path); strings.Contains(body, code) {
		t.Fatal("redeemed page echoes the one-time code")
	}
}

func TestDesktopLoginReuseFails(t *testing.T) {
	engine := newDesktopLoginEngine(t)
	const token = "owner-jwt-secret-value"
	path := mintDesktopLogin(t, token, "gid://axonhub/Project/4", "/channels")

	first := redeemDesktopLogin(engine, path, "127.0.0.1:12345")
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}

	second := redeemDesktopLogin(engine, path, "[::1]:12345")
	if second.Code != http.StatusNotFound {
		t.Fatalf("reuse status = %d, want 404", second.Code)
	}
	if strings.Contains(second.Body.String(), token) {
		t.Fatal("reused code returned the token")
	}
}

func TestDesktopLoginExpiryFails(t *testing.T) {
	engine := newDesktopLoginEngine(t)
	start := time.Unix(1_700_000_000, 0).UTC()
	desktopLogins.setClock(func() time.Time { return start })
	const token = "owner-jwt-secret-value"
	path := mintDesktopLogin(t, token, "gid://axonhub/Project/4", "/")

	desktopLogins.setClock(func() time.Time { return start.Add(desktopLoginTTL) })
	rec := redeemDesktopLogin(engine, path, "127.0.0.1:12345")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expired status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatal("expired code returned the token")
	}
}

func TestDesktopLoginRejectsNonLoopback(t *testing.T) {
	engine := newDesktopLoginEngine(t)
	const token = "owner-jwt-secret-value"
	path := mintDesktopLogin(t, token, "gid://axonhub/Project/4", "/desktop")

	for _, remote := range []string{"192.168.1.9:50000", "192.0.2.1:1234", ""} {
		rec := redeemDesktopLogin(engine, path, remote)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("remote %q status = %d, want 403", remote, rec.Code)
		}
		if strings.Contains(rec.Body.String(), token) {
			t.Fatalf("remote %q received the token", remote)
		}
	}

	// A rejected remote client must not consume the code.
	ok := redeemDesktopLogin(engine, path, "127.0.0.1:9")
	if ok.Code != http.StatusOK {
		t.Fatalf("loopback status after remote attempts = %d, want 200", ok.Code)
	}
	if !strings.Contains(ok.Body.String(), token) {
		t.Fatal("loopback redeem did not set the token")
	}
}

func TestMintDesktopLoginAcceptsRequestDetailPath(t *testing.T) {
	engine := newDesktopLoginEngine(t)
	const (
		token     = "owner-jwt-secret-value"
		projectID = "gid://axonhub/Project/4"
		nextPath  = "/project/requests/gid:%2F%2Faxonhub%2FRequest%2F1"
	)

	path := mintDesktopLogin(t, token, projectID, nextPath)
	rec := redeemDesktopLogin(engine, path, "127.0.0.1:12345")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	want := `location.replace(` + mustJSONString(t, nextPath) + `);`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body missing %s\n%s", want, rec.Body.String())
	}
}

func TestMintDesktopLoginRejectsOpenRedirect(t *testing.T) {
	resetDesktopLogins(t)
	const (
		token     = "owner-jwt-secret-value"
		projectID = "gid://axonhub/Project/4"
	)
	for _, nextPath := range []string{
		"https://evil.example/phish",
		"http://user@evil.example/phish",
		"//evil.example",
		"///evil.example",
		`/\evil.example`,
		`/\\evil.example`,
		"/@evil.example",
		"/%2f%2fevil.example",
		"/%5Cevil.example",
		"/%40evil.example",
		"javascript:alert(1)",
		"dashboard",
		"",
		"/ok\n//evil.example",
	} {
		_, err := MintDesktopLogin(token, projectID, nextPath)
		if !errors.Is(err, errDesktopLoginNextPath) {
			t.Fatalf("mint %q error = %v, want next-path rejection", nextPath, err)
		}
		if strings.Contains(err.Error(), token) {
			t.Fatalf("mint error contains the token: %v", err)
		}
	}
}

func newDesktopLoginEngine(t *testing.T) *gin.Engine {
	t.Helper()
	resetDesktopLogins(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerDesktopLoginRoute(engine.Group("/admin"))
	return engine
}

func resetDesktopLogins(t *testing.T) {
	t.Helper()
	desktopLogins.reset(nil)
	t.Cleanup(func() { desktopLogins.reset(nil) })
}

func mintDesktopLogin(t *testing.T, token, projectID, nextPath string) string {
	t.Helper()
	path, err := MintDesktopLogin(token, projectID, nextPath)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return path
}

func redeemDesktopLogin(engine *gin.Engine, rawPath, remote string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, rawPath, nil)
	req.RemoteAddr = remote
	engine.ServeHTTP(rec, req)
	return rec
}

func assertHandoffPath(t *testing.T, rawPath, token, projectID string) {
	t.Helper()
	parsed, err := url.Parse(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != DesktopLoginPath {
		t.Fatalf("path = %q, want %q", parsed.Path, DesktopLoginPath)
	}
	if len(parsed.Query()) != 1 || parsed.Query().Get(desktopLoginCodeQuery) == "" {
		t.Fatalf("handoff query = %q", parsed.RawQuery)
	}
	if strings.Contains(rawPath, token) || strings.Contains(rawPath, projectID) {
		t.Fatalf("handoff URL contains the session: %s", rawPath)
	}
}

func handoffCode(t *testing.T, rawPath string) string {
	t.Helper()
	parsed, err := url.Parse(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Query().Get(desktopLoginCodeQuery)
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
