package biz

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestRequestServiceLoadCompletedResponsesSessionCertification(t *testing.T) {
	for _, test := range []struct {
		name, request, response string
		complete                bool
	}{
		{"root", `{"input":"canary"}`, `{"id":"response","status":"completed","output":[]}`, true},
		{"unresolved ancestor", `{"previous_response_id":"missing","input":"delta"}`, `{"id":"response","status":"completed","output":[]}`, false},
		{"incomplete", `{"input":"canary"}`, `{"id":"response","status":"incomplete","output":[]}`, false},
		{"wrong provider ID", `{"input":"canary"}`, `{"id":"wrong","status":"completed","output":[]}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			client := enttest.NewEntClient(t, "sqlite3", "file:"+filepath.Join(t.TempDir(), "session.db")+"?_fk=1")
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			project, err := client.Project.Create().SetName("test").SetStatus("active").Save(ctx)
			require.NoError(t, err)
			key, err := client.APIKey.Create().SetKey("synthetic").SetName("test").SetProject(project).Save(ctx)
			require.NoError(t, err)
			_, err = client.DataStorage.Create().SetName("primary").SetDescription("synthetic database").SetPrimary(true).SetType("database").SetSettings(&objects.DataStorageSettings{}).SetStatus("active").Save(ctx)
			require.NoError(t, err)
			_, err = client.Request.Create().SetAPIKeyID(key.ID).SetProjectID(project.ID).SetModelID("gpt-5").SetFormat("openai/responses").SetExternalID("response").SetStatus("completed").SetRequestBody([]byte(test.request)).SetResponseBody([]byte(test.response)).Save(ctx)
			require.NoError(t, err)
			service := newResponsesSessionRequestService(client)
			ctx = contexts.WithProjectID(contexts.WithAPIKey(ctx, key), project.ID)
			// When
			request, _, found, err := service.LoadCompletedResponsesSession(ctx, "response")
			// Then
			require.NoError(t, err)
			require.Equal(t, test.complete, found)
			known, lookupErr := service.HasCompletedResponsesOrigin(ctx, "response")
			require.NoError(t, lookupErr)
			require.True(t, known)
			if found {
				require.Equal(t, []byte(test.request), request)
			}
		})
	}
}
