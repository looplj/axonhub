package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestOIDCHandlersGetBaseURLRequiresConfiguredPublicURL(t *testing.T) {
	h := &OIDCHandlers{}

	got, err := h.getBaseURL(context.Background())

	require.ErrorIs(t, err, errOIDCPublicURLRequired)
	require.Empty(t, got)
}

func TestOIDCHandlersGetBaseURLDoesNotUseRequestHost(t *testing.T) {
	h := &OIDCHandlers{publicURL: "https://axonhub.example.com/"}

	got, err := h.getBaseURL(context.Background())

	require.NoError(t, err)
	require.Equal(t, "https://axonhub.example.com", got)
}

func TestOIDCHandlersGetBaseURLWithBasePath(t *testing.T) {
	ctx := contexts.WithBasePath(context.Background(), "/services/axonhub")
	for publicURL, want := range map[string]string{
		"https://axonhub.example.com":                   "https://axonhub.example.com/services/axonhub",
		"https://axonhub.example.com/":                  "https://axonhub.example.com/services/axonhub",
		"https://axonhub.example.com/services/axonhub/": "https://axonhub.example.com/services/axonhub",
		"https://axonhub.example.com/public/":           "https://axonhub.example.com/public",
	} {
		t.Run(publicURL, func(t *testing.T) {
			h := &OIDCHandlers{publicURL: publicURL}
			got, err := h.getBaseURL(ctx)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func TestOIDCRedirectsWithBasePath(t *testing.T) {
	const publicURL = "https://axonhub.example.com"
	const basePath = "/services/axonhub"
	const callbackPath = "/oauth/oidc/callback"
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name        string
		redirectURL string
		want        string
	}{
		{name: "default", want: publicURL + basePath + callbackPath},
		{name: "override", redirectURL: "https://callback.example.com/oidc", want: "https://callback.example.com/oidc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.want, r.FormValue("redirect_uri"))
				http.Error(w, "invalid grant", http.StatusBadRequest)
			}))
			defer tokenEndpoint.Close()
			service, err := biz.NewOIDCService(biz.OIDCServiceParams{
				Config: biz.OIDCConfig{Providers: []biz.OIDCProvider{{
					ID:          "example",
					ClientID:    "test-client",
					AuthURL:     "https://idp.example.com/authorize",
					TokenURL:    tokenEndpoint.URL,
					RedirectURL: tc.redirectURL,
				}}},
				CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
			})
			require.NoError(t, err)
			h := &OIDCHandlers{oidc: service, publicURL: publicURL}
			router := gin.New()
			h.RegisterRoutes(router.Group("/oauth"))

			authorize := httptest.NewRequest(http.MethodGet, "/oauth/oidc/authorize/example", nil)
			authorize = authorize.WithContext(contexts.WithBasePath(authorize.Context(), basePath))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, authorize)
			require.Equal(t, http.StatusOK, response.Code)
			var payload struct {
				Data struct {
					URL   string `json:"url"`
					State string `json:"state"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			authorizeURL, err := url.Parse(payload.Data.URL)
			require.NoError(t, err)
			require.Equal(t, tc.want, authorizeURL.Query().Get("redirect_uri"))
			require.NotEmpty(t, payload.Data.State)

			callback := httptest.NewRequest(http.MethodGet, callbackPath+"?code=invalid&state="+payload.Data.State, nil)
			callback = callback.WithContext(contexts.WithBasePath(callback.Context(), basePath))
			response = httptest.NewRecorder()
			router.ServeHTTP(response, callback)
			require.Equal(t, http.StatusFound, response.Code)
			location, err := url.Parse(response.Header().Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "axonhub.example.com", location.Host)
			require.Equal(t, basePath+"/oauth/oidc/idp-callback", location.Path)
			require.Equal(t, "auth_failed", location.Query().Get("error"))
			require.Contains(t, location.Query().Get("error_description"), "invalid grant")
		})
	}
}

func TestOIDCHandlersGetBaseURLRejectsUnsafeConfiguration(t *testing.T) {
	for _, publicURL := range []string{
		"//attacker.example.com",
		"https://axonhub.example.com/callback?next=attacker",
		"https://user:secret@axonhub.example.com",
		"https://:443",
		"ftp://axonhub.example.com",
	} {
		t.Run(publicURL, func(t *testing.T) {
			h := &OIDCHandlers{publicURL: publicURL}

			_, err := h.getBaseURL(context.Background())

			require.Error(t, err)
		})
	}
}
