// 职责: 验证服务挂载路径，并将带有路径前缀的请求转发给内部路由。
// 关联文件:
//   conf/conf.go
//   internal/server/server.go
//   internal/contexts/context.go
// 注意事项: 只重写 Gin 自己生成的尾斜杠重定向。应用生成的重定向使用请求上下文中的前缀。

package server

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/felixge/httpsnoop"

	"github.com/looplj/axonhub/internal/contexts"
)

var mountPathPattern = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)+$`)

// NormalizeBasePath accepts a URL path such as /axonhub and canonicalizes root
// and trailing slashes for use in routing and public URLs.
func NormalizeBasePath(basePath string) (string, error) {
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath == "" {
		return "", nil
	}
	if !mountPathPattern.MatchString(basePath) || path.Clean(basePath) != basePath {
		return "", fmt.Errorf("invalid server.base_path %q: expected an absolute URL path such as /axonhub", basePath)
	}
	return basePath, nil
}

func mountBasePath(basePath string, next http.Handler) http.Handler {
	if basePath == "" {
		return next
	}

	mounted := http.StripPrefix(basePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Prefix") != "" {
			r.Header = r.Header.Clone()
			r.Header.Del("X-Forwarded-Prefix")
		}
		r = r.WithContext(contexts.WithBasePath(r.Context(), basePath))

		// Gin changes URL.Path in place when it redirects to add or remove a
		// trailing slash. Retain the mount path on those redirects. Preserve
		// the underlying writer's optional interfaces for SSE and WebSockets.
		originalPath := r.URL.Path
		wrapped := httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(writeHeader httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(status int) {
					location := w.Header().Get("Location")
					if (status == http.StatusMovedPermanently || status == http.StatusTemporaryRedirect) &&
						r.URL.Path != originalPath && strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") {
						w.Header().Set("Location", basePath+location)
					}
					writeHeader(status)
				}
			},
		})
		next.ServeHTTP(wrapped, r)
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.EscapedPath() == basePath:
			location := url.URL{Path: basePath + "/", RawQuery: r.URL.RawQuery, ForceQuery: r.URL.ForceQuery}
			http.Redirect(w, r, location.String(), http.StatusPermanentRedirect)
		case strings.HasPrefix(r.URL.Path, basePath+"/"):
			mounted.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}
