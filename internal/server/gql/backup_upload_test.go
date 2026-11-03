package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/server/backup"
	"github.com/looplj/axonhub/internal/server/biz"
)

const restoreUploadMutation = `mutation($file: Upload!) {
  restore(file: $file, input: {
    includeChannels: false,
    includeModels: false,
    includeAPIKeys: false,
    channelConflictStrategy: skip,
    modelConflictStrategy: skip,
    modelPriceConflictStrategy: skip,
    apiKeyConflictStrategy: skip
  }) { success message }
}`

// TestAdminGraphqlRestoreAcceptsLargeMultipartUpload drives the restore
// mutation through the real multipart transport with a payload above gqlgen's
// 32 MiB default, which is the shape that used to be refused before the
// mutation ran ("failed to parse multipart form, request body too large").
func TestAdminGraphqlRestoreAcceptsLargeMultipartUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = db.Close() })

	// Every mutation runs inside an entgql transaction, and restore opens its own
	// on top of it, so the pool has to be able to hand out a second connection.
	db.Driver().(*entsql.Driver).DB().SetMaxOpenConns(4)

	setupCtx := ent.NewContext(context.Background(), db)
	setupCtx = authz.WithTestBypass(setupCtx)

	hashed, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	owner := db.User.Create().
		SetEmail(fmt.Sprintf("owner-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashed).
		SetFirstName("Owner").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		SetIsOwner(true).
		SaveX(setupCtx)

	handler := NewGraphqlHandlers(Dependencies{
		Ent: db,
		BackupService: backup.NewBackupService(backup.BackupServiceParams{
			Ent:           db,
			SystemService: biz.NewSystemService(biz.SystemServiceParams{Ent: db}),
		}),
	})

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		// Authentication is covered elsewhere; this test is about the upload
		// transport and the resolver, so seed the owner directly.
		ctx := ent.NewContext(c.Request.Context(), db)
		ctx = authz.WithTestBypass(ctx)
		ctx = contexts.WithUser(ctx, owner)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.POST("/admin/graphql", func(c *gin.Context) { handler.Graphql.ServeHTTP(c.Writer, c.Request) })

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	payload := restoreUploadPayload(t, 33<<20)

	code, body := postRestoreUpload(t, server.URL, "backup.json", payload)
	require.Equal(t, http.StatusOK, code, "body: %s", body)
	require.NotContains(t, string(body), `"errors"`, "body: %s", body)
	require.Contains(t, string(body), `"success":true`, "body: %s", body)
}

// restoreUploadPayload returns a valid backup document of at least size bytes,
// padded through a system config value so the multipart body passes the 32 MiB
// default the transport used to apply.
func restoreUploadPayload(t *testing.T, size int) []byte {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"version": backup.BackupVersion,
		"system_configs": []map[string]string{
			{"key": "test.padding", "value": strings.Repeat("x", size)},
		},
		"channels": []any{},
		"models":   []any{},
	})
	require.NoError(t, err)

	return payload
}

func postRestoreUpload(t *testing.T, baseURL, filename string, payload []byte) (int, []byte) {
	t.Helper()

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	operations, err := json.Marshal(map[string]any{
		"query":     restoreUploadMutation,
		"variables": map[string]any{"file": nil},
	})
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("operations", string(operations)))
	require.NoError(t, writer.WriteField("map", `{"0":["variables.file"]}`))

	part, err := writer.CreateFormFile("0", filename)
	require.NoError(t, err)
	_, err = part.Write(payload)
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, baseURL+"/admin/graphql", &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, out
}
