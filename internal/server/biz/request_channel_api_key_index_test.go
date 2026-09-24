package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entrequest "github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// newRequestServiceForAPIKeyIndex builds the minimal service graph
// CreateRequestExecution needs.
func newRequestServiceForAPIKeyIndex(t *testing.T, client *ent.Client) *RequestService {
	t.Helper()

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Client:        client,
	})

	return NewRequestService(
		client,
		systemService.CacheConfig,
		systemService,
		usageLogService,
		dataStorageService,
		NewLiveStreamRegistry(),
	)
}

func TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_api_key_index?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	requestService := newRequestServiceForAPIKeyIndex(t, client)

	testCases := []struct {
		name      string
		apiKeys   []string
		usedKey   string
		wantIndex *int
	}{
		{
			name:      "first key of a multi-key channel is key1",
			apiKeys:   []string{"sk-first-1111", "sk-second-2222", "sk-third-3333"},
			usedKey:   "sk-first-1111",
			wantIndex: func() *int { v := 1; return &v }(),
		},
		{
			name:      "second key of a multi-key channel is key2",
			apiKeys:   []string{"sk-first-1111", "sk-second-2222", "sk-third-3333"},
			usedKey:   "sk-second-2222",
			wantIndex: func() *int { v := 2; return &v }(),
		},
		{
			name:      "last key of a multi-key channel keeps its position",
			apiKeys:   []string{"sk-first-1111", "sk-second-2222", "sk-third-3333"},
			usedKey:   "sk-third-3333",
			wantIndex: func() *int { v := 3; return &v }(),
		},
		{
			name:      "keys differing only in the last 4 characters are still distinguished",
			apiKeys:   []string{"sk-alpha-9999", "sk-bravo-9999"},
			usedKey:   "sk-bravo-9999",
			wantIndex: func() *int { v := 2; return &v }(),
		},
		{
			name:      "single key channel has nothing to disambiguate",
			apiKeys:   []string{"sk-only-1111"},
			usedKey:   "sk-only-1111",
			wantIndex: nil,
		},
		{
			name:      "used key from outside the configured list stays null",
			apiKeys:   []string{"sk-first-1111", "sk-second-2222"},
			usedKey:   "sk-not-configured-3333",
			wantIndex: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channelEntity, err := client.Channel.Create().
				SetName("channel-" + tc.name).
				SetType(channel.TypeOpenai).
				SetBaseURL("https://api.openai.com").
				SetCredentials(objects.ChannelCredentials{APIKeys: tc.apiKeys}).
				SetSupportedModels([]string{"gpt-4"}).
				SetDefaultTestModel("gpt-4").
				SetStatus(channel.StatusEnabled).
				Save(baseCtx)
			require.NoError(t, err)

			requestEntity, err := client.Request.Create().
				SetModelID("gpt-4").
				SetFormat(string(llm.APIFormatOpenAIChatCompletion)).
				SetRequestBody([]byte(`{"model":"gpt-4"}`)).
				SetStatus(entrequest.StatusProcessing).
				SetStream(false).
				Save(baseCtx)
			require.NoError(t, err)

			ctx := contexts.EnsureContainer(baseCtx)
			contexts.WithChannelAPIKey(ctx, tc.usedKey)

			exec, err := requestService.CreateRequestExecution(
				ctx,
				&Channel{Channel: channelEntity},
				"gpt-4",
				requestEntity,
				httpclient.Request{
					Body:      []byte(`{"model":"gpt-4"}`),
					APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				},
				llm.APIFormatOpenAIChatCompletion,
				false,
			)
			require.NoError(t, err)

			if tc.wantIndex == nil {
				require.Nil(t, exec.ChannelAPIKeyIndex)
			} else {
				require.NotNil(t, exec.ChannelAPIKeyIndex)
				require.Equal(t, *tc.wantIndex, *exec.ChannelAPIKeyIndex)
			}
		})
	}
}

// TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex_LegacySingleKeyField
// covers channels stored with the legacy single-value field, which
// GetAllAPIKeys folds into the same positional list.
func TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex_LegacySingleKeyField(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_api_key_index_legacy?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	requestService := newRequestServiceForAPIKeyIndex(t, client)

	channelEntity, err := client.Channel.Create().
		SetName("legacy-multi-key").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{
			APIKey:  "sk-legacy-1111",
			APIKeys: []string{"sk-second-2222"},
		}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(baseCtx)
	require.NoError(t, err)

	requestEntity, err := client.Request.Create().
		SetModelID("gpt-4").
		SetFormat(string(llm.APIFormatOpenAIChatCompletion)).
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(entrequest.StatusProcessing).
		SetStream(false).
		Save(baseCtx)
	require.NoError(t, err)

	// The legacy APIKey field is folded in first, so the second entry is key2.
	ctx := contexts.EnsureContainer(baseCtx)
	contexts.WithChannelAPIKey(ctx, "sk-second-2222")

	exec, err := requestService.CreateRequestExecution(
		ctx,
		&Channel{Channel: channelEntity},
		"gpt-4",
		requestEntity,
		httpclient.Request{
			Body:      []byte(`{"model":"gpt-4"}`),
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
		llm.APIFormatOpenAIChatCompletion,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, exec.ChannelAPIKeyIndex)
	require.Equal(t, 2, *exec.ChannelAPIKeyIndex)
}

// TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex_OAuthChannel
// asserts OAuth channels stay null: their credential list is not positional.
func TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex_OAuthChannel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_api_key_index_oauth?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	requestService := newRequestServiceForAPIKeyIndex(t, client)

	channelEntity, err := client.Channel.Create().
		SetName("oauth-channel").
		SetType(channel.TypeClaudecode).
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{
			APIKey: `{"access_token":"sk-oauth-1111","refresh_token":"refresh-token"}`,
		}).
		SetSupportedModels([]string{"claude-sonnet-4"}).
		SetDefaultTestModel("claude-sonnet-4").
		SetStatus(channel.StatusEnabled).
		Save(baseCtx)
	require.NoError(t, err)

	requestEntity, err := client.Request.Create().
		SetModelID("claude-sonnet-4").
		SetFormat(string(llm.APIFormatAnthropicMessage)).
		SetRequestBody([]byte(`{"model":"claude-sonnet-4"}`)).
		SetStatus(entrequest.StatusProcessing).
		SetStream(false).
		Save(baseCtx)
	require.NoError(t, err)

	ctx := contexts.EnsureContainer(baseCtx)
	contexts.WithChannelAPIKey(ctx, "sk-oauth-1111")

	exec, err := requestService.CreateRequestExecution(
		ctx,
		&Channel{Channel: channelEntity},
		"claude-sonnet-4",
		requestEntity,
		httpclient.Request{
			Body:      []byte(`{"model":"claude-sonnet-4"}`),
			APIFormat: string(llm.APIFormatAnthropicMessage),
		},
		llm.APIFormatAnthropicMessage,
		false,
	)
	require.NoError(t, err)
	require.Nil(t, exec.ChannelAPIKeyIndex)
}

// TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex_RetryDifferentKeys
// walks retries that fall over to another key and checks each execution keeps
// its own position.
func TestRequestService_CreateRequestExecution_ChannelAPIKeyIndex_RetryDifferentKeys(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_api_key_index_retry?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	requestService := newRequestServiceForAPIKeyIndex(t, client)

	channelEntity, err := client.Channel.Create().
		SetName("retry-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"sk-first-1111", "sk-second-2222"}}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(baseCtx)
	require.NoError(t, err)

	requestEntity, err := client.Request.Create().
		SetModelID("gpt-4").
		SetFormat(string(llm.APIFormatOpenAIChatCompletion)).
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(entrequest.StatusProcessing).
		SetStream(false).
		Save(baseCtx)
	require.NoError(t, err)

	for _, tc := range []struct {
		usedKey   string
		wantIndex int
	}{
		{usedKey: "sk-first-1111", wantIndex: 1},
		{usedKey: "sk-second-2222", wantIndex: 2},
	} {
		ctx := contexts.EnsureContainer(baseCtx)
		contexts.WithChannelAPIKey(ctx, tc.usedKey)

		exec, err := requestService.CreateRequestExecution(
			ctx,
			&Channel{Channel: channelEntity},
			"gpt-4",
			requestEntity,
			httpclient.Request{
				Body:      []byte(`{"model":"gpt-4"}`),
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
			},
			llm.APIFormatOpenAIChatCompletion,
			false,
		)
		require.NoError(t, err)
		require.NotNil(t, exec.ChannelAPIKeyIndex, "execution for key %s should record an index", tc.usedKey)
		require.Equal(t, tc.wantIndex, *exec.ChannelAPIKeyIndex)
	}
}
