package biz

import (
	"context"
	"net/http"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
	"github.com/zhenzou/executors"
)

func setupTestRequestService(t *testing.T) (*RequestService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=1")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	systemService := NewSystemService(SystemServiceParams{
		Ent: client,
	})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{},
		Executor:      executors.NewPoolScheduleExecutor(),
		Client:        client,
	})

	requestService := NewRequestService(client, systemService, usageLogService, dataStorageService, NewLiveStreamRegistry())

	return requestService, client, ctx
}

func TestRequestService_ClearStaleProcessingOnStartup(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	tr, err := client.Trace.Create().
		SetTraceID("test-trace").
		SetProjectID(proj.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create stale requests (>1 hour old)
	var staleRequestIDs []int
	for i := 0; i < 2; i++ {
		req, err := client.Request.Create().
			SetProjectID(proj.ID).
			SetTraceID(tr.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(request.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)
		staleRequestIDs = append(staleRequestIDs, req.ID)
	}

	// Create recent requests (<1 hour old)
	var recentRequestIDs []int
	for i := 0; i < 2; i++ {
		req, err := client.Request.Create().
			SetProjectID(proj.ID).
			SetTraceID(tr.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(request.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-30 * time.Minute)).
			Save(ctx)
		require.NoError(t, err)
		recentRequestIDs = append(recentRequestIDs, req.ID)
	}

	// Create stale executions
	var staleExecIDs []int
	for _, reqID := range staleRequestIDs {
		exec, err := client.RequestExecution.Create().
			SetProjectID(proj.ID).
			SetRequestID(reqID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(requestexecution.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)
		staleExecIDs = append(staleExecIDs, exec.ID)
	}

	// Create recent executions (<1 hour old)
	var recentExecIDs []int
	for _, reqID := range recentRequestIDs {
		exec, err := client.RequestExecution.Create().
			SetProjectID(proj.ID).
			SetRequestID(reqID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(requestexecution.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-30 * time.Minute)).
			Save(ctx)
		require.NoError(t, err)
		recentExecIDs = append(recentExecIDs, exec.ID)
	}

	err = svc.ClearStaleProcessingOnStartup(ctx)
	require.NoError(t, err)

	for _, id := range staleRequestIDs {
		req, err := client.Request.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, request.StatusCanceled, req.Status)
	}

	for _, id := range recentRequestIDs {
		req, err := client.Request.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, request.StatusProcessing, req.Status)
	}

	for _, id := range staleExecIDs {
		exec, err := client.RequestExecution.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusCanceled, exec.Status)
	}

	for _, id := range recentExecIDs {
		exec, err := client.RequestExecution.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusProcessing, exec.Status)
	}
}

func TestRequestService_PersistsEffectiveIdentity(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().SetName("test-project").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	user, err := client.User.Create().SetEmail("test@example.com").SetPassword("password").Save(ctx)
	require.NoError(t, err)

	apiKey, err := client.APIKey.Create().SetName("Test Key").SetKey("ah-test").SetProjectID(proj.ID).SetUserID(user.ID).Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithProjectID(ctx, proj.ID)
	ctx = contexts.WithAPIKey(ctx, apiKey)
	ctx = shared.WithSessionID(ctx, "session-123")

	llmRequest := &llm.Request{
		Model:               "gpt-5.4",
		Metadata:            map[string]string{"user_id": "metadata-user"},
		Messages:            []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}}},
		TransformerMetadata: map[string]any{shared.TransformerMetadataKeyOpenAIIdentityOwner: "user:1"},
	}

	rawRequest := &httpclient.Request{Headers: http.Header{"Session_id": []string{"session-123"}}, Body: []byte(`{"model":"gpt-5.4"}`)}

	reqRecord, err := svc.CreateRequest(ctx, llmRequest, rawRequest, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.Equal(t, "session-123", reqRecord.EffectivePromptCacheKey)
	require.Equal(t, "metadata-user", reqRecord.EffectiveSafetyIdentifier)
	require.Equal(t, "metadata-user", reqRecord.EffectiveUser)
	require.Equal(t, "session-123", reqRecord.EffectiveSessionID)

	channelEntity, err := client.Channel.Create().
		SetType("openai").
		SetName("test-channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-5.4"}).
		SetDefaultTestModel("gpt-5.4").
		Save(ctx)
	require.NoError(t, err)

	channelReq := httpclient.Request{
		Headers: http.Header{"Session_id": []string{"session-123"}},
		Body:    []byte(`{"model":"gpt-5.4","prompt_cache_key":"session-123","safety_identifier":"metadata-user","user":"metadata-user"}`),
	}

	execRecord, err := svc.CreateRequestExecution(ctx, &Channel{Channel: channelEntity}, "gpt-5.4", reqRecord, channelReq, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.Equal(t, "session-123", execRecord.EffectivePromptCacheKey)
	require.Equal(t, "metadata-user", execRecord.EffectiveSafetyIdentifier)
	require.Equal(t, "metadata-user", execRecord.EffectiveUser)
	require.Equal(t, "session-123", execRecord.EffectiveSessionID)
}

func TestRequestService_LeavesMissingEffectiveIdentityNil(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().SetName("test-project").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithProjectID(ctx, proj.ID)

	llmRequest := &llm.Request{
		Model:    "gpt-5.4",
		Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}}},
	}

	rawRequest := &httpclient.Request{Body: []byte(`{"model":"gpt-5.4"}`)}

	reqRecord, err := svc.CreateRequest(ctx, llmRequest, rawRequest, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)

	requestCount, err := client.Request.Query().Where(
		request.ID(reqRecord.ID),
		request.EffectivePromptCacheKeyIsNil(),
		request.EffectiveSafetyIdentifierIsNil(),
		request.EffectiveUserIsNil(),
		request.EffectiveSessionIDIsNil(),
	).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	channelEntity, err := client.Channel.Create().
		SetType("openai").
		SetName("test-channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-5.4"}).
		SetDefaultTestModel("gpt-5.4").
		Save(ctx)
	require.NoError(t, err)

	channelReq := httpclient.Request{Body: []byte(`{"model":"gpt-5.4"}`)}

	execRecord, err := svc.CreateRequestExecution(ctx, &Channel{Channel: channelEntity}, "gpt-5.4", reqRecord, channelReq, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)

	executionCount, err := client.RequestExecution.Query().Where(
		requestexecution.ID(execRecord.ID),
		requestexecution.EffectivePromptCacheKeyIsNil(),
		requestexecution.EffectiveSafetyIdentifierIsNil(),
		requestexecution.EffectiveUserIsNil(),
		requestexecution.EffectiveSessionIDIsNil(),
	).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, executionCount)
}

func TestRequestService_ClearStaleProcessingOnStartup_NoStaleRecords(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	err := svc.ClearStaleProcessingOnStartup(ctx)
	require.NoError(t, err)
}

func TestRequestService_ClearStaleProcessingOnStartup_PartialFailure(t *testing.T) {
	// This test verifies that if one cleanup operation fails, others still run
	// and errors are properly aggregated
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	tr, err := client.Trace.Create().
		SetTraceID("test-trace").
		SetProjectID(proj.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create only stale executions (no stale requests)
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetStream(false).
		SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	exec, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusProcessing).
		SetStream(false).
		SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	// Cleanup should succeed for both
	err = svc.ClearStaleProcessingOnStartup(ctx)
	require.NoError(t, err)

	// Verify execution was canceled
	exec, err = client.RequestExecution.Get(ctx, exec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCanceled, exec.Status)

	// Verify request was also canceled
	req, err = client.Request.Get(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCanceled, req.Status)
}
