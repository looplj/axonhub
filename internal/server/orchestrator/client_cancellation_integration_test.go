package orchestrator

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// These tests drive the real ChatCompletionOrchestrator pipeline so the client
// cancellation counter, the cooldown write, and the candidate-pool reload are all
// exercised end to end instead of calling the ChannelService helpers directly.

type cancellationTestEnv struct {
	ctx            context.Context
	client         *ent.Client
	project        *ent.Project
	channelService *biz.ChannelService
	systemService  *biz.SystemService
	orchestrator   *ChatCompletionOrchestrator
	channelIDs     []int
}

func createCancellationTestChannel(t *testing.T, ctx context.Context, client *ent.Client, name string, keys []string) *ent.Channel {
	t.Helper()

	row, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKeys: keys}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return row
}

func newCancellationTestEnv(t *testing.T, executor pipeline.Executor, keys []string) *cancellationTestEnv {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })

	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)
	project := createTestProject(t, ctx, client)

	rowA := createCancellationTestChannel(t, ctx, client, "cancel-channel-a", keys)
	rowB := createCancellationTestChannel(t, ctx, client, "cancel-channel-b", []string{"other-key"})

	systemService := newTestSystemService(client)
	channelService := biz.NewChannelService(biz.ChannelServiceParams{
		CacheConfig:     xcache.Config{Mode: xcache.ModeMemory},
		Ent:             client,
		SystemService:   systemService,
		HttpClient:      httpclient.NewHttpClient(),
		WebhookNotifier: biz.NewWebhookNotifier(systemService, httpclient.NewHttpClient()),
	})
	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
	}
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)
	requestService := biz.NewRequestService(client, systemService.CacheConfig, systemService, usageLogService, dataStorageService, biz.NewLiveStreamRegistry())

	require.NotEmpty(t, keys)
	outboundA, err := openai.NewOutboundTransformer(rowA.BaseURL, keys[0])
	require.NoError(t, err)
	outboundB, err := openai.NewOutboundTransformer(rowB.BaseURL, "other-key")
	require.NoError(t, err)

	candidates := channelsToTestCandidates([]*biz.Channel{
		{Channel: rowA, Outbound: outboundA},
		{Channel: rowB, Outbound: outboundB},
	}, "gpt-4")
	require.Len(t, candidates, 2)

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       &staticChannelSelector{candidates: candidates},
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares:           []pipeline.Middleware{stream.EnsureUsage()},
	}

	return &cancellationTestEnv{
		ctx:            ctx,
		client:         client,
		project:        project,
		channelService: channelService,
		systemService:  systemService,
		orchestrator:   orchestrator,
		channelIDs:     []int{rowA.ID, rowB.ID},
	}
}

func (e *cancellationTestEnv) useExecutor(executor pipeline.Executor) {
	e.orchestrator.PipelineFactory = pipeline.NewFactory(executor)
}

func (e *cancellationTestEnv) setPolicy(t *testing.T, policy biz.ClientCancellationPolicy) {
	t.Helper()

	require.NoError(t, e.systemService.SetRetryPolicy(e.ctx, &biz.RetryPolicy{
		Enabled:                  true,
		MaxChannelRetries:        1,
		MaxSingleChannelRetries:  0,
		RetryDelayMs:             0,
		ClientCancellationPolicy: policy,
	}))
}

func (e *cancellationTestEnv) process(projectID int, apiKey string) error {
	ctx := contexts.WithProjectID(e.ctx, projectID)
	if apiKey != "" {
		ctx = contexts.WithChannelAPIKey(ctx, apiKey)
	}

	_, err := e.orchestrator.Process(ctx, buildTestRequest("gpt-4", "hello", false))

	return err
}

func (e *cancellationTestEnv) getChannel(t *testing.T, id int) *ent.Channel {
	t.Helper()

	row, err := e.client.Channel.Get(e.ctx, id)
	require.NoError(t, err)

	return row
}

// waitForStatus polls until the channel reaches the wanted status, because the
// performance record is recorded by the asynchronous worker.
func (e *cancellationTestEnv) waitForStatus(t *testing.T, id int, want channel.Status) *ent.Channel {
	t.Helper()

	var last *ent.Channel

	require.Eventually(t, func() bool {
		row, err := e.client.Channel.Get(e.ctx, id)
		if err != nil {
			return false
		}

		last = row

		return row.Status == want
	}, 5*time.Second, 20*time.Millisecond)

	return last
}

func cancellationSwitchAndCooldownPolicy(threshold, cooldownMinutes int) biz.ClientCancellationPolicy {
	return biz.ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       threshold,
		CooldownMinutes: cooldownMinutes,
		Action:          biz.ClientCancellationActionSwitchAndCooldown,
		Scope:           biz.ClientCancellationScopeGlobal,
	}
}

func TestClientCancellationCooldown_ThresholdTakesChannelOutOfRotation(t *testing.T) {
	env := newCancellationTestEnv(t, &mockExecutor{err: context.Canceled}, []string{"key-1"})
	env.setPolicy(t, cancellationSwitchAndCooldownPolicy(2, 3))

	channelID := env.channelIDs[0]

	// First cancellation only increments the independent counter.
	require.Error(t, env.process(env.project.ID, "key-1"))
	require.Equal(t, channel.StatusEnabled, env.getChannel(t, channelID).Status)

	// The second one reaches the threshold and starts the cooldown.
	require.Error(t, env.process(env.project.ID, "key-1"))

	disabled := env.waitForStatus(t, channelID, channel.StatusDisabled)
	require.NotNil(t, disabled.AutoDisabledAt)
	require.NotNil(t, disabled.AutoDisableExpiresAt)
	require.WithinDuration(t, time.Now().Add(3*time.Minute), *disabled.AutoDisableExpiresAt, time.Minute)
	require.NotNil(t, disabled.ErrorMessage)
	require.Contains(t, *disabled.ErrorMessage, "Client cancellation cooldown")

	// The cooldown must remove the channel from the candidate pool so the next
	// request switches to the other channel. The cache reload is asynchronous.
	require.Eventually(t, func() bool {
		for _, ch := range env.channelService.GetEnabledChannels() {
			if ch.ID == channelID {
				return false
			}
		}

		return true
	}, 5*time.Second, 20*time.Millisecond)

	remaining := env.channelService.GetEnabledChannels()
	require.Len(t, remaining, 1)
	require.Equal(t, env.channelIDs[1], remaining[0].ID)
}

func TestClientCancellationCooldown_SuccessResetsCounter(t *testing.T) {
	env := newCancellationTestEnv(t, &mockExecutor{err: context.Canceled}, []string{"key-1"})
	env.setPolicy(t, cancellationSwitchAndCooldownPolicy(2, 3))

	channelID := env.channelIDs[0]

	require.Error(t, env.process(env.project.ID, "key-1"))

	// A successful request resets the streak to zero.
	env.useExecutor(&mockExecutor{response: &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": {"application/json"}},
		Body:       buildMockOpenAIResponse("resp-ok", "gpt-4", "hi", 1, 1),
	}})
	require.NoError(t, env.process(env.project.ID, "key-1"))

	// One more cancellation must not trigger the cooldown.
	env.useExecutor(&mockExecutor{err: context.Canceled})
	require.Error(t, env.process(env.project.ID, "key-1"))

	time.Sleep(300 * time.Millisecond)

	require.Equal(t, channel.StatusEnabled, env.getChannel(t, channelID).Status)
}

func TestClientCancellationCooldown_CooldownOnlyDisablesUsedCredential(t *testing.T) {
	env := newCancellationTestEnv(t, &mockExecutor{err: context.Canceled}, []string{"key-1", "key-2"})
	env.setPolicy(t, biz.ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       1,
		CooldownMinutes: 5,
		Action:          biz.ClientCancellationActionCooldownOnly,
		Scope:           biz.ClientCancellationScopeGlobal,
	})

	channelID := env.channelIDs[0]

	require.Error(t, env.process(env.project.ID, "key-1"))

	var row *ent.Channel

	require.Eventually(t, func() bool {
		current, err := env.client.Channel.Get(env.ctx, channelID)
		if err != nil {
			return false
		}

		row = current

		return len(current.DisabledAPIKeys) == 1
	}, 5*time.Second, 20*time.Millisecond)

	// The channel keeps serving with its other credential.
	require.Equal(t, channel.StatusEnabled, row.Status)
	require.Equal(t, "key-1", row.DisabledAPIKeys[0].Key)
	require.NotNil(t, row.DisabledAPIKeys[0].ExpiresAt)
	require.Contains(t, row.DisabledAPIKeys[0].Reason, "Client cancellation cooldown")
}

func TestClientCancellationCooldown_ProjectFirstOverride(t *testing.T) {
	env := newCancellationTestEnv(t, &mockExecutor{err: context.Canceled}, []string{"key-1"})
	env.setPolicy(t, biz.ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       5,
		CooldownMinutes: 30,
		Action:          biz.ClientCancellationActionSwitchAndCooldown,
		Scope:           biz.ClientCancellationScopeProjectFirst,
		ProjectOverrides: []biz.ClientCancellationProjectPolicy{
			{ProjectID: 42, Enabled: true, Threshold: 1, CooldownMinutes: 1},
		},
	})

	channelID := env.channelIDs[0]

	// A project without an override keeps the global threshold of 5.
	require.Error(t, env.process(env.project.ID, "key-1"))
	require.Equal(t, channel.StatusEnabled, env.getChannel(t, channelID).Status)

	// The overriding project cools the channel down on the first cancellation.
	require.Error(t, env.process(42, "key-1"))

	disabled := env.waitForStatus(t, channelID, channel.StatusDisabled)
	require.NotNil(t, disabled.AutoDisableExpiresAt)
	require.WithinDuration(t, time.Now().Add(time.Minute), *disabled.AutoDisableExpiresAt, 30*time.Second)
}

func TestClientCancellationCooldown_StatusCodeFailuresAreNotCancellations(t *testing.T) {
	failing := &mockExecutor{err: &httpclient.Error{
		Method:     http.MethodPost,
		URL:        "https://api.openai.com/v1/chat/completions",
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       []byte(`{"error":"boom"}`),
	}}
	env := newCancellationTestEnv(t, failing, []string{"key-1"})
	// A threshold of one makes any wrongly counted failure immediately visible.
	env.setPolicy(t, cancellationSwitchAndCooldownPolicy(1, 3))

	channelID := env.channelIDs[0]

	require.Error(t, env.process(env.project.ID, "key-1"))
	time.Sleep(300 * time.Millisecond)

	require.Equal(t, channel.StatusEnabled, env.getChannel(t, channelID).Status)
	// A 500 must still fail over to the next channel: the cancellation guard only
	// covers client cancellations.
	require.GreaterOrEqual(t, failing.requestCalls.Load(), int64(2))

	// The cancellation counter was never touched, so a threshold of two still
	// needs two real cancellations.
	env.setPolicy(t, cancellationSwitchAndCooldownPolicy(2, 3))
	env.useExecutor(&mockExecutor{err: context.Canceled})
	require.Error(t, env.process(env.project.ID, "key-1"))
	time.Sleep(300 * time.Millisecond)

	require.Equal(t, channel.StatusEnabled, env.getChannel(t, channelID).Status)
}
