package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
)

func TestLoadPanelReusesDesktopKey(t *testing.T) {
	const (
		token     = "desktop-token"
		projectID = "gid://axonhub/Project/1"
		enabled   = "sk-enabled-desktop-key"
		disabled  = "sk-disabled-desktop-key"
	)
	tests := []struct {
		name   string
		nodes  []desktopKeyNode
		secret string
	}{
		{
			name:   "enabled",
			secret: enabled,
			nodes: []desktopKeyNode{{
				ID: "gid://axonhub/APIKey/1", Name: desktopKeyName, Status: statusKeyEnabled, Secret: enabled,
			}},
		},
		{
			name:   "disabled only",
			secret: disabled,
			nodes: []desktopKeyNode{{
				ID: "gid://axonhub/APIKey/2", Name: desktopKeyName, Status: statusKeyDisabled, Secret: disabled,
			}},
		},
		{
			name:   "enabled ahead of disabled",
			secret: enabled,
			nodes: []desktopKeyNode{
				{ID: "gid://axonhub/APIKey/2", Name: desktopKeyName, Status: statusKeyDisabled, Secret: disabled},
				{ID: "gid://axonhub/APIKey/1", Name: desktopKeyName, Status: statusKeyEnabled, Secret: enabled},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, calls := serveDesktopPanel(t, desktopKeyListBody(t, tt.nodes), "", "")
			got, err := loadPanelSnapshot(t.Context(), server.URL, desktopSession{Token: token, ProjectID: projectID}, server.Client(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if got.DesktopKey != tt.secret {
				t.Fatal("stored key mismatch")
			}
			if lo.SomeBy(*calls, func(call postedGraphQL) bool {
				return call.OperationName == "CreateAPIKey" || strings.Contains(call.Query, "createAPIKey")
			}) {
				t.Fatal("createAPIKey was called")
			}
			keyCall, ok := lo.Find(*calls, func(call postedGraphQL) bool {
				return call.OperationName == "DesktopPanelKey"
			})
			if !ok {
				t.Fatal("desktop key was not loaded")
			}
			assertDesktopKeyQuery(t, keyCall, token, projectID)
		})
	}
}

func TestLoadPanelCreatesDesktopKey(t *testing.T) {
	const (
		token     = "desktop-token"
		projectID = "gid://axonhub/Project/7"
		secret    = "sk-created-desktop-key"
	)
	tests := []struct {
		name  string
		nodes []desktopKeyNode
	}{
		{name: "missing", nodes: []desktopKeyNode{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, calls := serveDesktopPanel(t, desktopKeyListBody(t, tt.nodes), createdDesktopKeyBody(t, secret), "")
			got, err := loadPanelSnapshot(t.Context(), server.URL, desktopSession{Token: token, ProjectID: projectID}, server.Client(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if got.DesktopKey != secret {
				t.Fatal("stored key mismatch")
			}
			create, ok := lo.Find(*calls, func(call postedGraphQL) bool {
				return call.OperationName == "CreateAPIKey"
			})
			if !ok {
				t.Fatal("createAPIKey was not called")
			}
			assertCreateDesktopKey(t, create, projectID)
		})
	}
}

func TestLoadPanelRestoresArchivedDesktopKey(t *testing.T) {
	const (
		token     = "desktop-token"
		projectID = "gid://axonhub/Project/3"
		secret    = "sk-archived-desktop-key"
		keyID     = "gid://axonhub/APIKey/3"
	)
	server, calls := serveDesktopPanel(t,
		desktopKeyListBody(t, []desktopKeyNode{{
			ID: keyID, Name: desktopKeyName, Status: statusKeyArchived, Secret: secret,
		}}),
		"",
		restoredDesktopKeyBody(t, keyID, secret),
	)
	got, err := loadPanelSnapshot(t.Context(), server.URL, desktopSession{Token: token, ProjectID: projectID}, server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.DesktopKey != secret {
		t.Fatal("stored key mismatch")
	}
	if lo.SomeBy(*calls, func(call postedGraphQL) bool { return call.OperationName == "CreateAPIKey" }) {
		t.Fatal("createAPIKey was called for an archived key")
	}
	restore, ok := lo.Find(*calls, func(call postedGraphQL) bool { return call.OperationName == "UpdateAPIKeyStatus" })
	if !ok {
		t.Fatal("the archived key was not enabled")
	}
	if restore.Variables["id"] != keyID || restore.Variables["status"] != statusKeyEnabled {
		t.Fatalf("restore variables = %#v", restore.Variables)
	}
}

func TestPresentDesktopKeyCopiesStoredKey(t *testing.T) {
	const secret = "sk-stored-desktop-key"
	action := presentDesktopKey(secret)
	if action.Copy != secret {
		t.Fatal("copy text is not the stored key")
	}
	if action.Label == secret || strings.Contains(action.Label, secret) {
		t.Fatal("row label is the stored key")
	}
	if action.Label != desktopKeyName {
		t.Fatalf("label = %q", action.Label)
	}
	empty := presentDesktopKey("")
	if empty.Copy != "" || empty.Label != desktopKeyName {
		t.Fatal("an empty key changed the row label")
	}
}

func TestDesktopKeyErrorOmitsSecret(t *testing.T) {
	const secret = "sk-super-secret-desktop-key"
	session := desktopSession{Token: "desktop-token", ProjectID: "gid://axonhub/Project/1"}
	tests := []struct {
		name       string
		keyStatus  int
		keyBody    string
		createCode int
		createBody string
	}{
		{name: "lookup http", keyStatus: http.StatusInternalServerError, keyBody: secret},
		{name: "lookup graphql", keyStatus: http.StatusOK, keyBody: `{"errors":[{"message":"rejected ` + secret + `"}]}`},
		{
			name: "create http", keyStatus: http.StatusOK, keyBody: `{"data":{"apiKeys":{"edges":[]}}}`,
			createCode: http.StatusInternalServerError, createBody: secret,
		},
		{
			name: "create graphql", keyStatus: http.StatusOK, keyBody: `{"data":{"apiKeys":{"edges":[]}}}`,
			createCode: http.StatusOK, createBody: `{"errors":[{"message":"rejected ` + secret + `"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call, err := readPostedGraphQL(r)
				if err != nil {
					t.Errorf("read graphql: %v", err)
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				switch call.OperationName {
				case "DesktopPanel":
					_, _ = io.WriteString(w, panelHTTPBody("UTC"))
				case "DesktopPanelFailures":
					_, _ = io.WriteString(w, `{"data":{"requests":{"totalCount":0}}}`)
				case "CreateAPIKey":
					w.WriteHeader(tt.createCode)
					_, _ = io.WriteString(w, tt.createBody)
				default:
					w.WriteHeader(tt.keyStatus)
					_, _ = io.WriteString(w, tt.keyBody)
				}
			}))
			t.Cleanup(server.Close)
			_, err := loadPanelSnapshot(t.Context(), server.URL, session, server.Client(), time.Now())
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatal("error includes the key")
			}
		})
	}
}

func TestFailedRefreshKeepsDesktopKey(t *testing.T) {
	shell := newShell("http://127.0.0.1:9", desktopSession{Token: "desktop-token", ProjectID: "gid://axonhub/Project/1"})
	shell.snapshot = panelSnapshot{RequestsToday: 4, DesktopKey: "previous-desktop-key"}
	shell.loadSeq.Store(1)
	shell.applySnapshot(1, panelSnapshot{RequestsToday: 9, DesktopKey: "next-desktop-key"}, errors.New("load desktop api key"))
	if shell.snapshot.DesktopKey != "previous-desktop-key" || shell.snapshot.RequestsToday != 4 {
		t.Fatal("refresh replaced the previous key")
	}
	if shell.loadErr != panelRefreshError {
		t.Fatalf("loadErr = %q", shell.loadErr)
	}
}

func assertCreateDesktopKey(t *testing.T, call postedGraphQL, projectID string) {
	t.Helper()
	if call.OperationName != "CreateAPIKey" || !strings.Contains(call.Query, "createAPIKey") {
		t.Fatalf("operation = %q", call.OperationName)
	}
	if len(call.Variables) != 1 {
		t.Fatalf("variables = %#v", call.Variables)
	}
	input, ok := call.Variables["input"].(map[string]any)
	if !ok {
		t.Fatalf("input = %#v", call.Variables["input"])
	}
	keys := lo.Keys(input)
	if len(keys) != 2 || !lo.Contains(keys, "name") || !lo.Contains(keys, "projectID") {
		t.Fatalf("input keys = %#v", keys)
	}
	if input["name"] != desktopKeyName || input["projectID"] != projectID {
		t.Fatalf("input = %#v", input)
	}
	for _, forbidden := range []string{"scopes", "profiles", "allowedIps", "type"} {
		if _, exists := input[forbidden]; exists || strings.Contains(call.Query, forbidden) {
			t.Fatalf("create includes %s", forbidden)
		}
	}
}

func serveDesktopPanel(t *testing.T, keyBody, createBody, restoreBody string) (*httptest.Server, *[]postedGraphQL) {
	t.Helper()
	calls := &[]postedGraphQL{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := readPostedGraphQL(r)
		if err != nil {
			t.Errorf("read graphql: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		*calls = append(*calls, call)
		body := panelHTTPBody("UTC")
		switch call.OperationName {
		case "DesktopPanelFailures":
			body = `{"data":{"requests":{"totalCount":0}}}`
		case "DesktopPanelKey":
			body = keyBody
		case "CreateAPIKey":
			if createBody == "" {
				t.Errorf("createAPIKey was called")
				body = `{"errors":[{"message":"unexpected create"}]}`
			} else {
				body = createBody
			}
		case "UpdateAPIKeyStatus":
			if restoreBody == "" {
				t.Errorf("updateAPIKeyStatus was called")
				body = `{"errors":[{"message":"unexpected restore"}]}`
			} else {
				body = restoreBody
			}
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func desktopKeyListBody(t *testing.T, nodes []desktopKeyNode) string {
	t.Helper()
	edges := lo.Map(nodes, func(node desktopKeyNode, _ int) any {
		return map[string]any{"node": map[string]string{
			"id":     node.ID,
			"name":   node.Name,
			"status": node.Status,
			"key":    node.Secret,
		}}
	})
	return mustGraphQLData(t, map[string]any{
		"apiKeys": map[string]any{"edges": edges},
	})
}

func restoredDesktopKeyBody(t *testing.T, id, secret string) string {
	t.Helper()
	return mustGraphQLData(t, map[string]any{
		"updateAPIKeyStatus": map[string]string{
			"id":     id,
			"name":   desktopKeyName,
			"status": statusKeyEnabled,
			"key":    secret,
		},
	})
}

func createdDesktopKeyBody(t *testing.T, secret string) string {
	t.Helper()
	return mustGraphQLData(t, map[string]any{
		"createAPIKey": map[string]string{
			"id":     "gid://axonhub/APIKey/9",
			"name":   desktopKeyName,
			"status": statusKeyEnabled,
			"key":    secret,
		},
	})
}

func mustGraphQLData(t *testing.T, data any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
