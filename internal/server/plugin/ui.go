package plugin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	_ "embed"

	"github.com/looplj/axonhub/internal/objects"
)

const (
	uiTokenTTL       = time.Hour
	uiTokenMACPrefix = "plugin-ui|"
	pluginUIBridge   = "_axonhub/bridge.js"
	// allow-forms is required for the submit event to fire in a sandboxed
	// frame; form-action 'none' keeps forms from navigating anywhere, so pages
	// still have no network path besides the bridge.
	pluginUICSP = "sandbox allow-scripts allow-forms; default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'none'; form-action 'none'; frame-ancestors 'self'"
)

//go:embed uisdk/bridge.js
var bridgeJS []byte

var errUITokenRejected = errors.New("plugin ui token rejected")

var pluginUIMIME = map[string]string{
	".js":    "text/javascript",
	".css":   "text/css",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".woff2": "font/woff2",
}

// uiTokenPayload is the canonical token body. Field order is the JSON order.
type uiTokenPayload struct {
	PluginID      string `json:"plugin_id"`
	PackageSHA256 string `json:"package_sha256"`
	Entry         string `json:"entry"`
	UserID        int    `json:"user_id"`
	Exp           int64  `json:"exp"`
}

// PluginUIRequest is one token-scoped UI resource read.
// Path is the gin wildcard, including its leading slash.
type PluginUIRequest struct {
	Manager   *Manager
	Secret    string
	SecretErr error
	Token     string
	Path      string
}

// IssueUIToken signs a one-hour token for an active plugin page entry.
// T-07 calls this from createPluginUISession. The token is the resource credential.
func (m *Manager) IssueUIToken(secret, pluginID, entry string, userID int, now time.Time) (string, error) {
	if userID <= 0 {
		return "", errors.New("plugin ui token user id must be positive")
	}
	if secret == "" {
		return "", errors.New("plugin ui token secret is required")
	}
	if m == nil {
		return "", errors.New("plugin manager is unavailable")
	}
	row := m.Current().ByPluginID(pluginID)
	if row == nil || !row.Enabled || row.Status != RuntimeStatusActive {
		return "", fmt.Errorf("plugin %q is not active", pluginID)
	}
	if !pluginEntryDeclared(row, entry) {
		return "", fmt.Errorf("entry %q is not declared", entry)
	}
	if row.PackageSHA256 == "" {
		return "", errors.New("plugin package is unavailable")
	}

	body, err := json.Marshal(uiTokenPayload{
		PluginID:      pluginID,
		PackageSHA256: row.PackageSHA256,
		Entry:         entry,
		UserID:        userID,
		Exp:           now.Add(uiTokenTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("encode plugin ui token: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	signature := base64.RawURLEncoding.EncodeToString(uiTokenMAC(secret, payload))
	return payload + "." + signature, nil
}

func pluginEntryDeclared(row *LoadedPlugin, entry string) bool {
	if row == nil || row.Manifest.UI == nil {
		return false
	}
	return slices.ContainsFunc(row.Manifest.UI.Pages, func(page objects.PluginUIPage) bool {
		return page.Entry == entry
	})
}

func uiTokenMAC(secret, payload string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(uiTokenMACPrefix + payload))
	return mac.Sum(nil)
}

func parseUIToken(secret, token string, now time.Time) (uiTokenPayload, error) {
	payloadPart, signaturePart, ok := strings.Cut(token, ".")
	if !ok || payloadPart == "" || signaturePart == "" || strings.Contains(signaturePart, ".") {
		return uiTokenPayload{}, errUITokenRejected
	}
	signature, err := base64.RawURLEncoding.DecodeString(signaturePart)
	if err != nil {
		return uiTokenPayload{}, errUITokenRejected
	}
	if !hmac.Equal(signature, uiTokenMAC(secret, payloadPart)) {
		return uiTokenPayload{}, errUITokenRejected
	}
	raw, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return uiTokenPayload{}, errUITokenRejected
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var payload uiTokenPayload
	if err := decoder.Decode(&payload); err != nil {
		return uiTokenPayload{}, errUITokenRejected
	}
	if decoder.More() {
		return uiTokenPayload{}, errUITokenRejected
	}
	if payload.UserID <= 0 || payload.PluginID == "" || payload.Entry == "" || payload.PackageSHA256 == "" || payload.Exp <= 0 {
		return uiTokenPayload{}, errUITokenRejected
	}
	if now.Unix() >= payload.Exp {
		return uiTokenPayload{}, errUITokenRejected
	}
	return payload, nil
}

// ServePluginUI handles GET and HEAD for one plugin UI resource.
// The caller adapts the router and supplies the already-resolved system secret.
func ServePluginUI(w http.ResponseWriter, r *http.Request, request PluginUIRequest) {
	applyPluginUIHeaders(w.Header())
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writePluginUIStatus(w, r, http.StatusMethodNotAllowed)
		return
	}
	if request.Manager == nil || request.SecretErr != nil {
		writePluginUIStatus(w, r, http.StatusServiceUnavailable)
		return
	}

	payload, err := parseUIToken(request.Secret, request.Token, time.Now())
	if err != nil {
		writePluginUIStatus(w, r, http.StatusForbidden)
		return
	}
	row := request.Manager.Current().ByPluginID(payload.PluginID)
	if row == nil || !row.Enabled || row.Status != RuntimeStatusActive || row.PackageSHA256 != payload.PackageSHA256 {
		writePluginUIStatus(w, r, http.StatusForbidden)
		return
	}
	if err := row.AcquireResource(); err != nil {
		writePluginUIStatus(w, r, http.StatusServiceUnavailable)
		return
	}
	defer row.ReleaseResource()

	cleaned, err := cleanPluginUIPath(request.Path)
	if err != nil {
		writePluginUIStatus(w, r, http.StatusForbidden)
		return
	}
	mimeType, bridge, ok := classifyPluginUIPath(cleaned, payload.Entry)
	if !ok {
		writePluginUIStatus(w, r, http.StatusForbidden)
		return
	}
	if bridge {
		w.Header().Set("Content-Type", mimeType)
		http.ServeContent(w, r, "bridge.js", time.Time{}, bytes.NewReader(bridgeJS))
		return
	}

	file, err := row.Open(cleaned)
	if err != nil {
		if errors.Is(err, ErrResourceUnavailable) || errors.Is(err, ErrGenerationClosed) {
			writePluginUIStatus(w, r, http.StatusServiceUnavailable)
			return
		}
		writePluginUIStatus(w, r, http.StatusNotFound)
		return
	}
	defer file.Close()

	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		writePluginUIStatus(w, r, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", mimeType)
	http.ServeContent(w, r, path.Base(cleaned), pluginUIModTime(file), seeker)
}

func classifyPluginUIPath(cleaned, entry string) (mime string, bridge bool, ok bool) {
	if cleaned == pluginUIBridge {
		return "text/javascript", true, true
	}
	if cleaned == entry {
		return "text/html", false, true
	}
	if isHTMLPath(cleaned) || !strings.HasPrefix(cleaned, "ui/") {
		return "", false, false
	}
	mime, ok = pluginUIMIME[path.Ext(cleaned)]
	if !ok {
		return "", false, false
	}
	return mime, false, true
}

func isHTMLPath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return true
	default:
		return false
	}
}

func cleanPluginUIPath(raw string) (string, error) {
	if raw == "" {
		return "", errUITokenRejected
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", errUITokenRejected
	}
	if strings.Contains(decoded, `\`) || strings.ContainsRune(decoded, 0) {
		return "", errUITokenRejected
	}
	if strings.HasPrefix(decoded, "/") {
		decoded = decoded[1:]
		if decoded == "" || strings.HasPrefix(decoded, "/") {
			return "", errUITokenRejected
		}
	}
	if decoded == "" || path.IsAbs(decoded) {
		return "", errUITokenRejected
	}
	if slices.Contains(strings.Split(decoded, "/"), "..") {
		return "", errUITokenRejected
	}
	cleaned := path.Clean(decoded)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) || strings.Contains(cleaned, `\`) {
		return "", errUITokenRejected
	}
	return cleaned, nil
}

func pluginUIModTime(file io.ReadCloser) time.Time {
	statter, ok := file.(interface{ Stat() (fs.FileInfo, error) })
	if !ok {
		return time.Time{}
	}
	info, err := statter.Stat()
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func applyPluginUIHeaders(header http.Header) {
	header.Set("Content-Security-Policy", pluginUICSP)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Access-Control-Allow-Origin", "*")
	header.Del("Access-Control-Allow-Credentials")
}

func writePluginUIStatus(w http.ResponseWriter, r *http.Request, status int) {
	applyPluginUIHeaders(w.Header())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(w, pluginUIStatusText(status))
}

func pluginUIStatusText(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not found\n"
	case http.StatusServiceUnavailable:
		return "unavailable\n"
	case http.StatusMethodNotAllowed:
		return "method not allowed\n"
	default:
		return "forbidden\n"
	}
}
