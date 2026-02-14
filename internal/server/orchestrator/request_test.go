package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPersistRequestMiddleware_OnOutboundLlmResponse_NilRequest(t *testing.T) {
	state := &PersistenceState{
		Request: nil,
	}

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	ctx := context.Background()
	resp := &llm.Response{ID: "resp-1"}

	result, err := middleware.OnOutboundLlmResponse(ctx, resp)

	require.NoError(t, err)
	require.Equal(t, resp, result)
}

func TestPersistRequestMiddleware_OnOutboundLlmResponse_NilResponse(t *testing.T) {
	state := &PersistenceState{
		Request: &ent.Request{ID: 1},
	}

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	ctx := context.Background()

	result, err := middleware.OnOutboundLlmResponse(ctx, nil)

	require.NoError(t, err)
	require.Nil(t, result)
}

func TestPersistRequestMiddleware_Name(t *testing.T) {
	middleware := &persistRequestMiddleware{}
	require.Equal(t, "persist-request", middleware.Name())
}

func TestPersistRequestMiddleware_UsageExtraction_EmbeddingResponse(t *testing.T) {
	t.Parallel()
	
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"text-embedding-3-small"}).
		SetDefaultTestModel("text-embedding-3-small").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:         1,
			ProjectID:  1,
			APIKeyID:   1,
			Source:     "test",
			Format:     "openai",
			ModelID:    "text-embedding-3-small",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "text-embedding-3-small",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID: "resp-1",
		Embedding: &llm.EmbeddingResponse{
			Usage: &llm.EmbeddingUsage{
				PromptTokens: 100,
				TotalTokens:  100,
			},
		},
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
	require.NotNil(t, result.Embedding)
	require.Equal(t, int64(100), result.Embedding.Usage.PromptTokens)
}

func TestPersistRequestMiddleware_UsageExtraction_ChatResponse(t *testing.T) {
	t.Parallel()
	
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:         1,
			ProjectID:  1,
			APIKeyID:   1,
			Source:     "test",
			Format:     "openai",
			ModelID:    "gpt-4",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "gpt-4",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID: "resp-2",
		Usage: &llm.Usage{
			PromptTokens:     50,
			CompletionTokens: 150,
			TotalTokens:      200,
		},
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
	require.NotNil(t, result.Usage)
	require.Equal(t, int64(50), result.Usage.PromptTokens)
	require.Equal(t, int64(150), result.Usage.CompletionTokens)
	require.Equal(t, int64(200), result.Usage.TotalTokens)
}

func TestPersistRequestMiddleware_UsageExtraction_NilUsage(t *testing.T) {
	t.Parallel()
	
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:         1,
			ProjectID:  1,
			APIKeyID:   1,
			Source:     "test",
			Format:     "openai",
			ModelID:    "gpt-4",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "gpt-4",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID:   "resp-3",
		Usage: nil,
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
}

func TestPersistRequestMiddleware_UsageExtraction_EmbeddingWithNilUsage(t *testing.T) {
	t.Parallel()
	
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"text-embedding-3-small"}).
		SetDefaultTestModel("text-embedding-3-small").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:         1,
			ProjectID:  1,
			APIKeyID:   1,
			Source:     "test",
			Format:     "openai",
			ModelID:    "text-embedding-3-small",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "text-embedding-3-small",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID: "resp-4",
		Embedding: &llm.EmbeddingResponse{
			Usage: nil,
		},
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
}
