package gql

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/plugin"
)

// JSON may encode each body byte as a six-byte Unicode escape. Keep the
// envelope bounded while leaving the decoded 1 MiB body limit to Manager.Fetch.
const pluginFetchJSONLimit = 6*(1<<20) + (64 << 10)

type pluginFetchHandler struct {
	manager *plugin.Manager
}

func newPluginFetchHandler(manager *plugin.Manager) http.Handler {
	return &pluginFetchHandler{manager: manager}
}

func (h *pluginFetchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	manifestID, ok := pluginFetchID(r)
	if !ok {
		writeFetchCode(w, http.StatusNotFound, "NOT_FOUND", "plugin not found")
		return
	}
	user, ok := contexts.GetUser(r.Context())
	if !ok || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	row := activePluginUI(h.manager, manifestID)
	if row == nil {
		writeFetchCode(w, http.StatusNotFound, "NOT_FOUND", "plugin not found")
		return
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, pluginFetchJSONLimit+1))
	if err != nil {
		writeFetchCode(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body is invalid")
		return
	}
	if len(payload) > pluginFetchJSONLimit {
		writeFetchCode(w, http.StatusRequestEntityTooLarge, "LIMIT_EXCEEDED", "encoded fetch request is too large")
		return
	}
	var body struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || strings.TrimSpace(body.URL) == "" {
		writeFetchCode(w, http.StatusBadRequest, "INVALID_ARGUMENT", "url is required")
		return
	}

	result, err := h.manager.Fetch(r.Context(), append([]string(nil), row.GrantedNetwork...), plugin.FetchRequest{
		URL:     body.URL,
		Method:  body.Method,
		Headers: body.Headers,
		Body:    body.Body,
	})
	if err != nil {
		writeFetchFailure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  result.Status,
		"headers": result.Headers,
		"body":    result.Body,
	})
}

func pluginFetchID(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	const suffix = "/fetch"
	path := r.URL.Path
	if !strings.HasSuffix(path, suffix) {
		return "", false
	}
	trimmed := strings.TrimSuffix(path, suffix)
	const marker = "/plugins/"
	_, after, ok := strings.CutLast(trimmed, marker)
	if !ok {
		return "", false
	}
	if after == "" || strings.Contains(after, "/") {
		return "", false
	}
	decoded, err := url.PathUnescape(after)
	if err != nil || decoded == "" || strings.Contains(decoded, "/") {
		return "", false
	}
	return decoded, true
}

func writeFetchFailure(w http.ResponseWriter, _ *http.Request, err error) {
	var kind *plugin.FetchError
	if !errors.As(err, &kind) {
		writeFetchCode(w, http.StatusInternalServerError, "INTERNAL", "outbound request failed")
		return
	}
	if kind.Code == "CANCELED" {
		return
	}
	switch kind.Code {
	case "PERMISSION_DENIED":
		writeFetchCode(w, http.StatusForbidden, "FORBIDDEN", kind.Message)
	case "DEADLINE_EXCEEDED":
		writeFetchCode(w, http.StatusGatewayTimeout, "TIMEOUT", kind.Message)
	case "UNAVAILABLE":
		writeFetchCode(w, http.StatusBadGateway, "NETWORK_ERROR", kind.Message)
	case "INVALID_STATE":
		writeFetchCode(w, http.StatusConflict, "INVALID_STATE", kind.Message)
	case "LIMIT_EXCEEDED":
		writeFetchCode(w, http.StatusRequestEntityTooLarge, "LIMIT_EXCEEDED", kind.Message)
	case "INVALID_ARGUMENT":
		writeFetchCode(w, http.StatusBadRequest, "INVALID_ARGUMENT", kind.Message)
	case "CANCELED":
		return
	default:
		writeFetchCode(w, http.StatusInternalServerError, "INTERNAL", kind.Message)
	}
}

func writeFetchCode(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}
