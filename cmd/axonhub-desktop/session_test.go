package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin/binding"

	"github.com/looplj/axonhub/internal/server/api"
)

func TestOwnerProfilePassesInitializeBinding(t *testing.T) {
	req := api.InitializeSystemRequest{
		OwnerEmail:     ownerEmail,
		OwnerPassword:  "0123456789abcdef",
		OwnerFirstName: ownerFirstName,
		OwnerLastName:  ownerLastName,
		BrandName:      brandName,
	}
	if err := binding.Validator.ValidateStruct(req); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDesktopSessionInitializesOnce(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), ownerSecretName)
	projectID := "gid://axonhub/Project/7"
	var initialized bool
	var savedPassword string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/admin/system/status":
			_ = json.NewEncoder(w).Encode(api.SystemStatusResponse{IsInitialized: initialized})
		case "/admin/system/initialize":
			var req api.InitializeSystemRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("decode initialize: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req.OwnerEmail != ownerEmail || req.OwnerFirstName != ownerFirstName || req.OwnerLastName != ownerLastName || req.BrandName != brandName {
				t.Errorf("unexpected initialize profile: %#v", req)
			}
			if len(req.OwnerPassword) < 6 {
				t.Error("initialize password is shorter than the API requires")
			}
			savedPassword = req.OwnerPassword
			initialized = true
			_ = json.NewEncoder(w).Encode(api.InitializeSystemResponse{Success: true, Message: "System initialized successfully"})
		case "/admin/auth/signin":
			var req api.SignInRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("decode sign-in: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req.Email != ownerEmail || req.Password != savedPassword {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"message":"Invalid email or password"}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(api.SignInResponse{Token: "desktop-token"})
		case "/admin/graphql":
			if r.Header.Get("Authorization") != "Bearer desktop-token" {
				t.Errorf("authorization = %q", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"myProjects":[{"id":"` + projectID + `","name":"AxonHub Desktop"},{"id":"gid://axonhub/Project/8","name":"Default"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	var ensured []bool
	ensure := func(_ context.Context, secret string, fresh bool) error {
		if secret != savedPassword {
			t.Error("ensure got another secret than the one initialize used")
		}
		ensured = append(ensured, fresh)
		return nil
	}

	ctx := t.Context()
	first, err := openDesktopSession(ctx, server.URL, secretPath, server.Client(), ensure)
	if err != nil {
		t.Fatal(err)
	}
	if first.Token != "desktop-token" || first.ProjectID != projectID {
		t.Fatalf("session = %+v", first)
	}

	second, err := openDesktopSession(ctx, server.URL, secretPath, server.Client(), ensure)
	if err != nil {
		t.Fatal(err)
	}
	if second.ProjectID != projectID || second.Token != "desktop-token" {
		t.Fatalf("reused session = %+v", second)
	}
	stored, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stored)) != savedPassword {
		t.Fatal("stored owner secret does not match the password used to initialize")
	}
	if len(ensured) != 2 || !ensured[0] || ensured[1] {
		t.Fatalf("ensure fresh flags = %v, want [true false]", ensured)
	}
}

func TestOpenDesktopSessionAdoptsInitializedDatabase(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), ownerSecretName)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/system/status":
			_ = json.NewEncoder(w).Encode(api.SystemStatusResponse{IsInitialized: true})
		case "/admin/system/initialize":
			t.Error("an initialized database must not be initialized again")
		case "/admin/auth/signin":
			_ = json.NewEncoder(w).Encode(api.SignInResponse{Token: "desktop-token"})
		case "/admin/graphql":
			_, _ = w.Write([]byte(`{"data":{"myProjects":[{"id":"gid://axonhub/Project/3","name":"AxonHub Desktop"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	var gotSecret string
	session, err := openDesktopSession(t.Context(), server.URL, secretPath, server.Client(), func(_ context.Context, secret string, fresh bool) error {
		if fresh {
			t.Error("an initialized database is not fresh")
		}
		gotSecret = secret
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal("a missing owner secret must be created for an initialized database")
	}
	if gotSecret == "" || strings.TrimSpace(string(stored)) != gotSecret || session.ProjectID != "gid://axonhub/Project/3" {
		t.Fatalf("session = %+v", session)
	}
}

func TestDefaultProjectIDUsesGraphQLString(t *testing.T) {
	id, err := defaultProjectID([]projectRef{
		{ID: "gid://axonhub/Project/2", Name: "Other"},
		{ID: "gid://axonhub/Project/4", Name: desktopProjectName},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "gid://axonhub/Project/4" {
		t.Fatalf("project id = %q", id)
	}
	if _, err := defaultProjectID([]projectRef{{ID: "4", Name: desktopProjectName}}); err == nil {
		t.Fatal("numeric project id was accepted")
	}
	if _, err := defaultProjectID(nil); err == nil {
		t.Fatal("missing default project was accepted")
	}
}

func TestSessionPreloadScriptSetsUIStorageKeys(t *testing.T) {
	script, err := sessionPreloadScript(desktopSession{
		Token:     `tok"en`,
		ProjectID: "gid://axonhub/Project/4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `localStorage.setItem("axonhub_access_token", "tok\"en")`) {
		t.Fatalf("token was not stored as a JavaScript string: %s", script)
	}
	if !strings.Contains(script, `localStorage.setItem("axonhub_selected_project_id", "gid://axonhub/Project/4")`) {
		t.Fatalf("project id was not stored: %s", script)
	}
}

func TestLoopbackHost(t *testing.T) {
	if !loopbackHost("127.0.0.1") || !loopbackHost("::1") || !loopbackHost("localhost") {
		t.Fatal("loopback addresses were rejected")
	}
	if loopbackHost("0.0.0.0") || loopbackHost("192.168.1.1") || loopbackHost("") {
		t.Fatal("non-loopback host was accepted")
	}
}

func testJWT(exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp.Unix())))
	return header + "." + payload + ".sig"
}

func TestRenewDesktopSession(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	projectID := "gid://axonhub/Project/4"
	secretPath := filepath.Join(t.TempDir(), ownerSecretName)
	if err := os.WriteFile(secretPath, []byte("desktop-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		token      string
		refresh    int
		refreshed  string
		want       string
		wantSignIn bool
	}{
		{name: "fresh", token: testJWT(now.Add(20 * 24 * time.Hour)), want: testJWT(now.Add(20 * 24 * time.Hour))},
		{name: "refresh", token: testJWT(now.Add(24 * time.Hour)), refresh: http.StatusOK, refreshed: "renewed-token", want: "renewed-token"},
		{name: "not yet", token: testJWT(now.Add(24 * time.Hour)), refresh: http.StatusNoContent, want: testJWT(now.Add(24 * time.Hour))},
		{name: "refresh down", token: testJWT(now.Add(24 * time.Hour)), refresh: http.StatusInternalServerError, want: testJWT(now.Add(24 * time.Hour))},
		{name: "expired", token: testJWT(now.Add(-time.Hour)), refresh: http.StatusUnauthorized, want: "signed-in-token", wantSignIn: true},
		{name: "unreadable", token: "not-a-jwt", refresh: http.StatusUnauthorized, want: "signed-in-token", wantSignIn: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var signedIn bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/admin/auth/refresh":
					if tt.refresh == 0 {
						t.Error("refresh was called")
					}
					if tt.refresh == http.StatusOK {
						_ = json.NewEncoder(w).Encode(api.RefreshResponse{Token: tt.refreshed})
						return
					}
					w.WriteHeader(tt.refresh)
				case "/admin/auth/signin":
					signedIn = true
					var req api.SignInRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Errorf("decode sign-in: %v", err)
					}
					if req.Email != ownerEmail || req.Password != "desktop-secret" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					_ = json.NewEncoder(w).Encode(api.SignInResponse{Token: "signed-in-token"})
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			t.Cleanup(server.Close)

			got, err := renewDesktopSession(t.Context(), server.Client(), server.URL, secretPath, desktopSession{Token: tt.token, ProjectID: projectID}, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.Token != tt.want || got.ProjectID != projectID {
				t.Fatalf("session = %+v, want token %q", got, tt.want)
			}
			if signedIn != tt.wantSignIn {
				t.Fatalf("signed in = %v, want %v", signedIn, tt.wantSignIn)
			}
		})
	}
}

func TestWaitUntilListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := waitUntilListening(t.Context(), ln.Addr().String(), nil); err != nil {
		ln.Close()
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	listenErr := make(chan error, 1)
	listenErr <- errors.New("bind: address already in use")
	err = waitUntilListening(t.Context(), addr, listenErr)
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("error = %v", err)
	}
}

func TestRedactRemovesSecret(t *testing.T) {
	if got := redact("failed password secret-value", "secret-value"); got != "failed password [redacted]" {
		t.Fatalf("redact = %q", got)
	}
}
