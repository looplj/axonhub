package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const (
	// DesktopLoginPath is the GET route that redeems a one-time desktop login.
	// Mount it on the unauthenticated /admin group: the browser has no session yet.
	DesktopLoginPath = "/admin/desktop-login"

	desktopLoginRoute     = "/desktop-login"
	desktopLoginCodeQuery = "code"
	desktopLoginTTL       = 60 * time.Second
	desktopLoginCodeBytes = 32
)

var (
	errDesktopLoginIdentity = errors.New("desktop login requires a token and project id")
	errDesktopLoginNextPath = errors.New("desktop login next path must be one relative path")
)

// MintDesktopLogin binds token and projectID to an unguessable single-use code
// that expires after 60 seconds, and returns the handoff path
// /admin/desktop-login?code=... The token is not part of that path.
// nextPath must stay on this origin. In the raw form and after up to two
// unescapes it must be non-empty and start with one slash. It must not be
// protocol-relative (a leading //), a scheme, a host, or userinfo (@), and
// must not contain a backslash, a control character, or a space. "//" later
// in the path is allowed, so an encoded request id is a valid next path.
// cmd/axonhub-desktop calls this and opens the path on the loopback origin;
// GET DesktopLoginPath redeems it.
func MintDesktopLogin(token, projectID, nextPath string) (string, error) {
	if token == "" || projectID == "" {
		return "", errDesktopLoginIdentity
	}
	if !validDesktopNextPath(nextPath) {
		return "", errDesktopLoginNextPath
	}
	code, err := desktopLogins.mint(token, projectID, nextPath)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	query.Set(desktopLoginCodeQuery, code)
	return DesktopLoginPath + "?" + query.Encode(), nil
}

// registerDesktopLoginRoute mounts the redeem handler on the /admin group.
// The group must be the unauthenticated admin group so the path stays
// DesktopLoginPath and the timeout middleware still applies.
func registerDesktopLoginRoute(admin *gin.RouterGroup) {
	admin.GET(desktopLoginRoute, serveDesktopLogin)
}

func serveDesktopLogin(c *gin.Context) {
	// RemoteAddr is the TCP peer. ClientIP() honors X-Forwarded-For for trusted
	// proxies, so a remote caller could otherwise claim to be loopback.
	if !isLoopbackRemote(c.Request.RemoteAddr) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	grant, ok := desktopLogins.redeem(c.Query(desktopLoginCodeQuery))
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	page, err := desktopLoginPage(grant)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	header := c.Writer.Header()
	header.Set("Cache-Control", "no-store")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func desktopLoginPage(grant desktopLoginGrant) (string, error) {
	tokenJS, err := jsString(grant.token)
	if err != nil {
		return "", err
	}
	projectJS, err := jsString(grant.projectID)
	if err != nil {
		return "", err
	}
	nextJS, err := jsString(grant.nextPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"></head><body><script>
localStorage.setItem("axonhub_access_token", %s);
localStorage.setItem("axonhub_selected_project_id", %s);
location.replace(%s);
</script></body></html>`, tokenJS, projectJS, nextJS), nil
}

func jsString(value string) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode desktop login page: %w", err)
	}
	return string(encoded), nil
}

func validDesktopNextPath(path string) bool {
	forms, err := desktopNextPathForms(path)
	if err != nil {
		return false
	}
	return lo.EveryBy(forms, desktopNextPathShape)
}

func desktopNextPathForms(path string) ([]string, error) {
	forms := []string{path}
	current := path
	for range 2 {
		next, err := url.PathUnescape(current)
		if err != nil {
			return nil, err
		}
		if next == current {
			break
		}
		forms = append(forms, next)
		current = next
	}
	return forms, nil
}

func desktopNextPathShape(path string) bool {
	if path == "" || path[0] != '/' || strings.HasPrefix(path, "//") {
		return false
	}
	// "//" is only protocol-relative at the start. A later "//", including the
	// one inside an encoded gid:// request id, stays on this origin.
	if lo.SomeBy([]string{`\`, "@"}, func(part string) bool {
		return strings.Contains(path, part)
	}) {
		return false
	}
	if strings.ContainsFunc(path, func(r rune) bool {
		return r < 0x20 || r == 0x7f || unicode.IsSpace(r)
	}) {
		return false
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Scheme != "" || parsed.User != nil {
		return false
	}
	return parsed.Path != "" && parsed.Path[0] == '/' && !strings.HasPrefix(parsed.Path, "//")
}

type desktopLoginGrant struct {
	token     string
	projectID string
	nextPath  string
	expiresAt time.Time
}

type desktopLoginStore struct {
	mu     sync.Mutex
	grants map[string]desktopLoginGrant
	now    func() time.Time
}

func newDesktopLoginStore() *desktopLoginStore {
	return &desktopLoginStore{
		grants: map[string]desktopLoginGrant{},
		now:    time.Now,
	}
}

var desktopLogins = newDesktopLoginStore()

func (s *desktopLoginStore) mint(token, projectID, nextPath string) (string, error) {
	buf := make([]byte, desktopLoginCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate desktop login code: %w", err)
	}
	code := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, grant := range s.grants {
		if !now.Before(grant.expiresAt) {
			delete(s.grants, key)
		}
	}
	s.grants[code] = desktopLoginGrant{
		token:     token,
		projectID: projectID,
		nextPath:  nextPath,
		expiresAt: now.Add(desktopLoginTTL),
	}
	return code, nil
}

// redeem compares code with every stored code in constant time, then deletes
// the match. A second redeem and an expired code both fail. The code and token
// are never logged.
func (s *desktopLoginStore) redeem(code string) (desktopLoginGrant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	codeBytes := []byte(code)
	var matchedKey string
	var matched desktopLoginGrant
	found := 0
	for key, grant := range s.grants {
		if subtle.ConstantTimeCompare([]byte(key), codeBytes) == 1 {
			matchedKey = key
			matched = grant
			found = 1
		}
	}
	if found != 1 {
		return desktopLoginGrant{}, false
	}
	delete(s.grants, matchedKey)
	if !s.now().Before(matched.expiresAt) {
		return desktopLoginGrant{}, false
	}
	return matched, true
}

func (s *desktopLoginStore) setClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now == nil {
		s.now = time.Now
		return
	}
	s.now = now
}

func (s *desktopLoginStore) reset(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grants = map[string]desktopLoginGrant{}
	if now == nil {
		s.now = time.Now
		return
	}
	s.now = now
}
