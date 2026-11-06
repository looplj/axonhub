package gql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPluginAPIKeyQuotaUsagesAuthorizedProfile(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_profile_quota")
	key := fx.db.APIKey.Query().OnlyX(fx.setupCtx)
	quota := &objects.APIKeyQuota{TotalTokens: lo.ToPtr(int64(1000)), Period: objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime}}
	profiles := &objects.APIKeyProfiles{ActiveProfile: "limited", Profiles: []objects.APIKeyProfile{{Name: "limited", Quota: quota}}}
	key = fx.db.APIKey.UpdateOneID(key.ID).SetProfiles(profiles).SaveX(fx.setupCtx)
	req := fx.db.Request.Create().SetProjectID(key.ProjectID).SetAPIKeyID(key.ID).SetModelID("quota-model").SetRequestBody([]byte(`{}`)).SetStream(false).SetStatus(request.StatusCompleted).SaveX(fx.setupCtx)
	fx.db.UsageLog.Create().SetRequestID(req.ID).SetProjectID(key.ProjectID).SetAPIKeyID(key.ID).SetModelID("quota-model").SetPromptTokens(111).SetTotalTokens(111).SaveX(fx.setupCtx)
	otherProject := fx.db.Project.Create().SetName("other-quota-project").SaveX(fx.setupCtx)
	otherKey := fx.db.APIKey.Create().SetName("other-quota-key").SetKey("other-quota-key-secret").SetUserID(key.UserID).SetProjectID(otherProject.ID).SetType(apikey.TypeServiceAccount).SetProfiles(profiles).SaveX(fx.setupCtx)
	installed, err := fx.manager.Install(fx.setupCtx, pluginEndpointZip(t, "acme.quota", []string{"read_api_keys"}, true), nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"read_api_keys"}))
	post := func(token string, id int) (map[string]any, string) {
		body, err := json.Marshal(map[string]any{"query": fmt.Sprintf(`{ apiKeyQuotaUsages(apiKeyId: "gid://axonhub/APIKey/%d") { profileName usage { requestCount totalTokens } } }`, id)})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/admin/plugins/acme.quota/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Project-Id", fmt.Sprintf("gid://axonhub/Project/%d", key.ProjectID))
		recorder := httptest.NewRecorder()
		fx.engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var payload map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload, recorder.Body.String()
	}
	payload, raw := post(fx.ownerToken, key.ID)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, `"profileName":"limited"`)
	require.Contains(t, raw, `"requestCount":1`)
	require.Contains(t, raw, `"totalTokens":111`)
	payload, raw = post(fx.ownerToken, otherKey.ID)
	require.Contains(t, payload, "errors", raw)
	payload, raw = post(fx.readerToken, key.ID)
	require.Contains(t, payload, "errors", raw)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, nil))
	payload, raw = post(fx.ownerToken, key.ID)
	require.Contains(t, payload, "errors", raw)
}
