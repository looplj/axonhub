package biz

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
)

func newRequestStickyTestService(
	t *testing.T,
	systemCacheConfig xcache.Config,
	requestCacheConfig xcache.Config,
) (context.Context, *ent.Client, *RequestService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:request_sticky_"+time.Now().Format("20060102150405.000000000")+"?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: systemCacheConfig,
		Ent:         client,
	})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   systemCacheConfig,
		Client:        client,
	})

	return ctx, client, NewRequestService(client, requestCacheConfig, systemService, usageLogService, dataStorageService, NewLiveStreamRegistry())
}

func createRequestStickyScope(t *testing.T, ctx context.Context, client *ent.Client) (*ent.Thread, *ent.Trace, *ent.Channel) {
	t.Helper()

	projectEntity, err := client.Project.Create().
		SetName("request-sticky-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	threadEntity, err := client.Thread.Create().
		SetProjectID(projectEntity.ID).
		SetThreadID("request-sticky-thread").
		Save(ctx)
	require.NoError(t, err)

	traceEntity, err := client.Trace.Create().
		SetProjectID(projectEntity.ID).
		SetThreadID(threadEntity.ID).
		SetTraceID("request-sticky-trace").
		Save(ctx)
	require.NoError(t, err)

	channelEntity, err := client.Channel.Create().
		SetName("request-sticky-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return threadEntity, traceEntity, channelEntity
}

func createStickyRequest(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	traceID int,
	channelID int,
	status request.Status,
	createdAt time.Time,
) *ent.Request {
	t.Helper()

	req, err := client.Request.Create().
		SetProjectID(1).
		SetTraceID(traceID).
		SetChannelID(channelID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(status).
		SetStream(false).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)

	return req
}

func TestRequestService_PreviousChannelIsCachedAtSelection(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	ctx, client, service := newRequestStickyTestService(t, cacheConfig, cacheConfig)
	threadEntity, traceEntity, channelEntity := createRequestStickyScope(t, ctx, client)
	req := createStickyRequest(t, ctx, client, traceEntity.ID, channelEntity.ID, request.StatusProcessing, time.Now())

	require.NoError(t, service.UpdateRequestChannelID(ctx, req.ID, channelEntity.ID))

	channelID, err := service.GetPreviousChannelID(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.Equal(t, channelEntity.ID, channelID)

	channelID, err = service.GetPreviousChannelIDByThread(ctx, threadEntity.ID)
	require.NoError(t, err)
	require.Equal(t, channelEntity.ID, channelID)
}

func TestRequestService_PreviousChannelCacheMissDoesNotQueryHistoricalRequests(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	ctx, client, service := newRequestStickyTestService(t, cacheConfig, cacheConfig)
	threadEntity, traceEntity, channelEntity := createRequestStickyScope(t, ctx, client)
	createStickyRequest(t, ctx, client, traceEntity.ID, channelEntity.ID, request.StatusProcessing, time.Now())

	channelID, err := service.GetPreviousChannelID(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.Zero(t, channelID)

	channelID, err = service.GetPreviousChannelIDByThread(ctx, threadEntity.ID)
	require.NoError(t, err)
	require.Zero(t, channelID)
}

func TestRequestService_PreviousChannelUsesRedisCache(t *testing.T) {
	mr := miniredis.RunT(t)
	systemCacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	requestCacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}
	ctx, client, service := newRequestStickyTestService(t, systemCacheConfig, requestCacheConfig)
	threadEntity, traceEntity, channelEntity := createRequestStickyScope(t, ctx, client)
	req := createStickyRequest(t, ctx, client, traceEntity.ID, channelEntity.ID, request.StatusProcessing, time.Now())

	require.NoError(t, service.UpdateRequestChannelID(ctx, req.ID, channelEntity.ID))
	require.True(t, mr.Exists(buildPreviousTraceChannelCacheKey(traceEntity.ID)))
	require.True(t, mr.Exists(buildPreviousThreadChannelCacheKey(threadEntity.ID)))
	require.Equal(t, 30*time.Minute, mr.TTL(buildPreviousTraceChannelCacheKey(traceEntity.ID)))
	require.Equal(t, 30*time.Minute, mr.TTL(buildPreviousThreadChannelCacheKey(threadEntity.ID)))

	channelID, err := service.GetPreviousChannelID(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.Equal(t, channelEntity.ID, channelID)
}
