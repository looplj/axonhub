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

func TestRequestService_CreateRequestExecution_ChannelAPIKeySuffix(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_channel_api_key_suffix?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Client:        client,
	})
	requestService := NewRequestService(
		client,
		systemService.CacheConfig,
		systemService,
		usageLogService,
		dataStorageService,
		NewLiveStreamRegistry(),
	)

	channelEntity, err := client.Channel.Create().
		SetName("test-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
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

	testCases := []struct {
		name       string
		apiKey     *string
		wantSuffix *string
	}{
		{
			name:       "normal long API key saves last 4 characters",
			apiKey:     func() *string { s := "sk-test-abcdefghijkl-ZxWL"; return &s }(),
			wantSuffix: func() *string { s := "ZxWL"; return &s }(),
		},
		{
			name:       "boundary key with length 5 saves last 4 characters",
			apiKey:     func() *string { s := "abcde"; return &s }(),
			wantSuffix: func() *string { s := "bcde"; return &s }(),
		},
		{
			name:       "unicode key saves last 4 runes",
			apiKey:     func() *string { s := "key-密钥-9999"; return &s }(),
			wantSuffix: func() *string { s := "9999"; return &s }(),
		},
		{
			name:       "multi-byte unicode suffix saves last 4 runes",
			apiKey:     func() *string { s := "sk-provider-渠道密钥"; return &s }(),
			wantSuffix: func() *string { s := "渠道密钥"; return &s }(),
		},
		{
			name:       "different long key saves distinct suffix",
			apiKey:     func() *string { s := "sk-another-provider-key-1234"; return &s }(),
			wantSuffix: func() *string { s := "1234"; return &s }(),
		},
		{
			name:       "no channel API key in context leaves suffix nil",
			apiKey:     nil,
			wantSuffix: nil,
		},
		{
			name:       "short key with length 4 leaves suffix nil",
			apiKey:     func() *string { s := "abcd"; return &s }(),
			wantSuffix: nil,
		},
		{
			name:       "short key with length less than 4 leaves suffix nil",
			apiKey:     func() *string { s := "abc"; return &s }(),
			wantSuffix: nil,
		},
		{
			name:       "empty key leaves suffix nil",
			apiKey:     func() *string { s := ""; return &s }(),
			wantSuffix: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := contexts.EnsureContainer(baseCtx)
			if tc.apiKey != nil {
				contexts.WithChannelAPIKey(ctx, *tc.apiKey)
			}

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

			if tc.wantSuffix == nil {
				require.Nil(t, exec.ChannelAPIKeySuffix)
			} else {
				require.NotNil(t, exec.ChannelAPIKeySuffix)
				require.Equal(t, *tc.wantSuffix, *exec.ChannelAPIKeySuffix)
			}
		})
	}
}

func TestRequestService_CreateRequestExecution_ChannelAPIKeySuffix_RetryDifferentKeys(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_channel_api_key_suffix_retry?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Client:        client,
	})
	requestService := NewRequestService(
		client,
		systemService.CacheConfig,
		systemService,
		usageLogService,
		dataStorageService,
		NewLiveStreamRegistry(),
	)

	channelEntity, err := client.Channel.Create().
		SetName("multi-key-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
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

	// Simulate single orchestration container across retries with different selected keys
	sharedCtx := contexts.EnsureContainer(baseCtx)

	// First execution with Key A
	contexts.WithChannelAPIKey(sharedCtx, "sk-test-key-alpha-1111")
	exec1, err := requestService.CreateRequestExecution(
		sharedCtx,
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
	require.NotNil(t, exec1.ChannelAPIKeySuffix)
	require.Equal(t, "1111", *exec1.ChannelAPIKeySuffix)

	// Retry on another key B in the same container
	contexts.WithChannelAPIKey(sharedCtx, "sk-test-key-bravo-2222")
	exec2, err := requestService.CreateRequestExecution(
		sharedCtx,
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
	require.NotNil(t, exec2.ChannelAPIKeySuffix)
	require.Equal(t, "2222", *exec2.ChannelAPIKeySuffix)

	// Verify both executions hold their respective, distinct suffixes
	require.NotEqual(t, *exec1.ChannelAPIKeySuffix, *exec2.ChannelAPIKeySuffix)
}
