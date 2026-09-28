package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestAuthHandlers_RefreshErrorClassification(t *testing.T) {
	for _, scenario := range []struct {
		name string
		want int
	}{
		{name: "invalid token", want: http.StatusUnauthorized},
		{name: "missing user", want: http.StatusUnauthorized},
		{name: "inactive user", want: http.StatusUnauthorized},
		{name: "missing secret", want: http.StatusInternalServerError},
		{name: "secret lookup failure", want: http.StatusInternalServerError},
		{name: "user lookup failure", want: http.StatusInternalServerError},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given
			client := enttest.NewEntClient(t, "sqlite3", "file:auth-refresh?mode=memory&_fk=1")
			t.Cleanup(func() { _ = client.Close() })
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			_, err := client.System.Create().SetKey(biz.SystemKeySecretKey).SetValue("test-secret").Save(ctx)
			require.NoError(t, err)
			service := &biz.AuthService{
				SystemService: &biz.SystemService{Cache: xcache.NewFromConfig[ent.System](xcache.Config{})},
				UserService:   &biz.UserService{UserCache: xcache.NewFromConfig[ent.User](xcache.Config{})},
			}
			u, err := client.User.Create().SetEmail("refresh@example.com").SetPassword("test").SetStatus(user.StatusActivated).Save(ctx)
			require.NoError(t, err)
			now := time.Now()
			authTime := now.Add(-25 * 24 * time.Hour)
			token, err := service.GenerateJWTTokenAt(ctx, u, authTime, authTime)
			require.NoError(t, err)
			switch scenario.name {
			case "invalid token":
				token = "invalid"
			case "missing user":
				err = client.User.DeleteOne(u).Exec(ctx)
			case "inactive user":
				_, err = client.User.UpdateOne(u).SetStatus(user.StatusDeactivated).Save(ctx)
			case "missing secret":
				_, err = client.System.Delete().Exec(ctx)
			case "secret lookup failure":
				err = client.Close()
			case "user lookup failure":
				client.User.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
					return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
						return nil, context.DeadlineExceeded
					})
				}))
			}
			require.NoError(t, err)
			handler := &AuthHandlers{AuthService: service}
			router := gin.New()
			router.POST("/admin/auth/refresh", handler.Refresh)
			req := httptest.NewRequest(http.MethodPost, "/admin/auth/refresh", nil).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()

			// When
			router.ServeHTTP(recorder, req)

			// Then
			require.Equal(t, scenario.want, recorder.Code)
			if scenario.want == http.StatusInternalServerError {
				require.Contains(t, recorder.Body.String(), "Internal server error")
			}
		})
	}
}
