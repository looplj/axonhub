package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
)

func TestRequestServiceLoadCompletedResponsesSessionScopesByAPIKeyAndProject(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:responses_session_"+time.Now().Format("20060102150405.000000000")+"?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	projectEntity, err := client.Project.Create().
		SetName("responses-session-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	ownerKey, err := client.APIKey.Create().
		SetKey("responses-session-owner").
		SetName("owner").
		SetProject(projectEntity).
		Save(ctx)
	require.NoError(t, err)
	otherKey, err := client.APIKey.Create().
		SetKey("responses-session-other").
		SetName("other").
		SetProject(projectEntity).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.DataStorage.Create().
		SetName("primary").
		SetDescription("primary database storage").
		SetPrimary(true).
		SetType("database").
		SetSettings(new(objects.DataStorageSettings)).
		SetStatus("active").
		Save(ctx)
	require.NoError(t, err)

	createSessionRequest := func(apiKeyID int, format, responseID, marker string) {
		t.Helper()
		_, createErr := client.Request.Create().
			SetAPIKeyID(apiKeyID).
			SetProjectID(projectEntity.ID).
			SetModelID("gpt-5").
			SetFormat(format).
			SetRequestBody([]byte(`{"model":"gpt-5","input":"` + marker + `"}`)).
			SetResponseBody([]byte(`{"id":"` + responseID + `","status":"completed","output":[{"type":"message","role":"assistant","content":"` + marker + `"}]}`)).
			SetExternalID(responseID).
			SetStatus("completed").
			SetStream(true).
			Save(ctx)
		require.NoError(t, createErr)
	}
	createSessionRequest(ownerKey.ID, string(llm.APIFormatOpenAIResponse), "resp_shared", "owner")
	createSessionRequest(otherKey.ID, string(llm.APIFormatOpenAIResponse), "resp_shared", "other")
	createSessionRequest(ownerKey.ID, string(llm.APIFormatOpenAIResponseWebSocket), "resp_websocket", "websocket")

	service := newResponsesSessionRequestService(client)

	ownerCtx := contexts.WithProjectID(contexts.WithAPIKey(ctx, ownerKey), projectEntity.ID)
	requestBody, responseBody, found, err := service.LoadCompletedResponsesSession(ownerCtx, "resp_shared")
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, `{"model":"gpt-5","input":"owner"}`, string(requestBody))
	require.JSONEq(t, `{"id":"resp_shared","status":"completed","output":[{"type":"message","role":"assistant","content":"owner"}]}`, string(responseBody))

	otherCtx := contexts.WithProjectID(contexts.WithAPIKey(ctx, otherKey), projectEntity.ID)
	requestBody, responseBody, found, err = service.LoadCompletedResponsesSession(otherCtx, "resp_shared")
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, `{"model":"gpt-5","input":"other"}`, string(requestBody))
	require.JSONEq(t, `{"id":"resp_shared","status":"completed","output":[{"type":"message","role":"assistant","content":"other"}]}`, string(responseBody))

	differentProjectCtx := contexts.WithProjectID(contexts.WithAPIKey(ctx, ownerKey), projectEntity.ID+1)
	_, _, found, err = service.LoadCompletedResponsesSession(differentProjectCtx, "resp_shared")
	require.NoError(t, err)
	require.False(t, found)

	requestBody, responseBody, found, err = service.LoadCompletedResponsesSession(ownerCtx, "resp_websocket")
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, `{"model":"gpt-5","input":"websocket"}`, string(requestBody))
	require.JSONEq(t, `{"id":"resp_websocket","status":"completed","output":[{"type":"message","role":"assistant","content":"websocket"}]}`, string(responseBody))

	_, _, found, err = service.LoadCompletedResponsesSession(otherCtx, "resp_websocket")
	require.NoError(t, err)
	require.False(t, found)
	_, _, found, err = service.LoadCompletedResponsesSession(differentProjectCtx, "resp_websocket")
	require.NoError(t, err)
	require.False(t, found)

	_, _, found, err = service.LoadCompletedResponsesSession(ctx, "resp_shared")
	require.NoError(t, err)
	require.False(t, found)
}

func TestRequestServiceLoadCompletedResponsesSessionRejectsEmptyBodies(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:responses_session_empty_"+time.Now().Format("20060102150405.000000000")+"?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	projectEntity, err := client.Project.Create().
		SetName("responses-session-empty-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	apiKey, err := client.APIKey.Create().
		SetKey("responses-session-empty").
		SetName("empty").
		SetProject(projectEntity).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.DataStorage.Create().
		SetName("primary").
		SetDescription("primary database storage").
		SetPrimary(true).
		SetType("database").
		SetSettings(new(objects.DataStorageSettings)).
		SetStatus("active").
		Save(ctx)
	require.NoError(t, err)

	createSessionRequest := func(responseID string, requestBody, responseBody []byte) {
		t.Helper()
		_, createErr := client.Request.Create().
			SetAPIKeyID(apiKey.ID).
			SetProjectID(projectEntity.ID).
			SetModelID("gpt-5").
			SetFormat(string(llm.APIFormatOpenAIResponse)).
			SetRequestBody(requestBody).
			SetResponseBody(responseBody).
			SetExternalID(responseID).
			SetStatus("completed").
			SetStream(true).
			Save(ctx)
		require.NoError(t, createErr)
	}
	createSessionRequest("resp_empty_request", []byte(`{}`), []byte(`{"id":"resp_empty_request","status":"completed","output":[]}`))
	createSessionRequest("resp_empty_response", []byte(`{"model":"gpt-5","input":"hello"}`), []byte(`{}`))

	service := newResponsesSessionRequestService(client)

	lookupCtx := contexts.WithProjectID(contexts.WithAPIKey(ctx, apiKey), projectEntity.ID)
	for _, responseID := range []string{"resp_empty_request", "resp_empty_response"} {
		_, _, found, loadErr := service.LoadCompletedResponsesSession(lookupCtx, responseID)
		require.NoError(t, loadErr)
		require.False(t, found)
	}
}

func newResponsesSessionRequestService(client *ent.Client) *RequestService {
	var cacheConfig xcache.Config
	cacheConfig.Mode = xcache.ModeMemory

	systemParams := new(SystemServiceParams)
	systemParams.CacheConfig = cacheConfig
	systemParams.Ent = client
	systemService := NewSystemService(*systemParams)

	dataStorageParams := new(DataStorageServiceParams)
	dataStorageParams.SystemService = systemService
	dataStorageParams.CacheConfig = cacheConfig
	dataStorageParams.Client = client
	dataStorageService := NewDataStorageService(*dataStorageParams)
	channelService := NewChannelServiceForTest(client)

	return NewRequestService(
		client,
		cacheConfig,
		systemService,
		NewUsageLogService(client, systemService, channelService),
		dataStorageService,
		NewLiveStreamRegistry(),
	)
}
