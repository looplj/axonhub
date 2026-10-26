package gql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestClaudeCodeSafeguardsGraphQLSurface(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer db.Close()
	r := &Resolver{client: db, systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: db})}
	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: r}))
	execute := func(query string, scopes []string) map[string]any {
		payload, err := json.Marshal(map[string]string{"query": query})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		ctx := authz.NewUserContext(ent.NewContext(req.Context(), db), 123)
		ctx = contexts.WithUser(ctx, &ent.User{ID: 123, Scopes: scopes})
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req.WithContext(ctx))
		var result map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result), recorder.Body.String())
		return result
	}
	read := `query { claudeCodeSafeguardsSettings { usingServerDefaults defaultPolicy config { classifierModel policy fallbackModels escalateOnDenial attemptTimeoutSeconds timeoutSeconds maxTokens transcriptCharBudget } } }`
	write := `mutation { updateClaudeCodeSafeguardsSettings(input:{classifierModel:"reviewer",policy:"Custom policy",fallbackModels:["backup"],escalateOnDenial:true,attemptTimeoutSeconds:0.5,timeoutSeconds:12,maxTokens:1024,transcriptCharBudget:8000}) }`
	reset := `mutation { resetClaudeCodeSafeguardsSettings }`
	access := []string{"read_settings", "write_settings"}
	require.Contains(t, execute(read, nil), "errors")
	require.Contains(t, execute(write, []string{"read_settings"}), "errors")
	require.Contains(t, execute(reset, []string{"read_settings"}), "errors")
	require.NotContains(t, execute(write, access), "errors")
	result := execute(read, access)
	require.NotContains(t, result, "errors")
	settings := result["data"].(map[string]any)["claudeCodeSafeguardsSettings"].(map[string]any)
	require.Equal(t, false, settings["usingServerDefaults"])
	require.NotEmpty(t, settings["defaultPolicy"])
	config := settings["config"].(map[string]any)
	require.Equal(t, "Custom policy", config["policy"])
	require.Equal(t, "reviewer", config["classifierModel"])
	require.Equal(t, []any{"backup"}, config["fallbackModels"])
	require.Equal(t, 0.5, config["attemptTimeoutSeconds"])
	require.Equal(t, 12.0, config["timeoutSeconds"])
	require.Equal(t, 1024.0, config["maxTokens"])
	require.Equal(t, 8000.0, config["transcriptCharBudget"])
	require.Equal(t, true, config["escalateOnDenial"])
	require.NotContains(t, execute(reset, access), "errors")
	result = execute(read, access)
	require.Equal(t, true, result["data"].(map[string]any)["claudeCodeSafeguardsSettings"].(map[string]any)["usingServerDefaults"])
}
