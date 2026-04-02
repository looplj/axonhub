package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	openaitypes "github.com/looplj/axonhub/llm/transformer/openai"
)

func setupOpenAIRetrieveTest(t *testing.T) (*ent.Client, *biz.ChannelService, *gin.Engine, context.Context) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	channelSvc := biz.NewChannelServiceForTest(client)
	systemSvc := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	modelSvc := biz.NewModelService(biz.ModelServiceParams{
		ChannelService: channelSvc,
		SystemService:  systemSvc,
		Ent:            client,
	})

	handlers := &OpenAIHandlers{
		ModelService: modelSvc,
		EntClient:    client,
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := ent.NewContext(c.Request.Context(), client)
		ctx = authz.WithTestBypass(ctx)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/v1/models/*model", handlers.RetrieveModel)

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	return client, channelSvc, router, ctx
}

func TestOpenAIHandlers_RetrieveModel_SupportsSlashModelIDs(t *testing.T) {
	client, channelSvc, router, ctx := setupOpenAIRetrieveTest(t)

	createdAt := time.Unix(1712345678, 0)
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("DeepSeek Channel").
		SetBaseURL("https://api.deepseek.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"deepseek-chat"}).
		SetDefaultTestModel("deepseek-chat").
		SetSettings(&objects.ChannelSettings{ExtraModelPrefix: "deepseek"}).
		SetStatus(channel.StatusEnabled).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)

	channelSvc.SetEnabledChannelsForTest([]*biz.Channel{{Channel: ch}})

	req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek/deepseek-chat", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var got OpenAIModel
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Equal(t, "deepseek/deepseek-chat", got.ID)
	require.Equal(t, "model", got.Object)
	require.Equal(t, createdAt.Unix(), got.Created)
	require.Equal(t, "openai", got.OwnedBy)
}

func TestOpenAIHandlers_RetrieveModel_FallsBackToBasicWhenConfiguredMetadataMissing(t *testing.T) {
	client, channelSvc, router, ctx := setupOpenAIRetrieveTest(t)

	createdAt := time.Unix(1712345688, 0)
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("OpenAI Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"gpt-4o-mini"}).
		SetDefaultTestModel("gpt-4o-mini").
		SetStatus(channel.StatusEnabled).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)

	channelSvc.SetEnabledChannelsForTest([]*biz.Channel{{Channel: ch}})

	req := httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4o-mini?include=all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var got OpenAIModel
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Equal(t, "gpt-4o-mini", got.ID)
	require.Equal(t, "model", got.Object)
	require.Equal(t, createdAt.Unix(), got.Created)
	require.Equal(t, "openai", got.OwnedBy)
	require.Empty(t, got.Name)
	require.Nil(t, got.Capabilities)
	require.Nil(t, got.Pricing)
}

func TestOpenAIHandlers_RetrieveModel_ReturnsExtendedConfiguredModel(t *testing.T) {
	client, channelSvc, router, ctx := setupOpenAIRetrieveTest(t)

	channelCreatedAt := time.Unix(1712345698, 0)
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("OpenAI Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"gpt-4.1"}).
		SetDefaultTestModel("gpt-4.1").
		SetStatus(channel.StatusEnabled).
		SetCreatedAt(channelCreatedAt).
		Save(ctx)
	require.NoError(t, err)

	channelSvc.SetEnabledChannelsForTest([]*biz.Channel{{Channel: ch}})

	remark := "GPT-4.1 reasoning model"
	modelCreatedAt := time.Unix(1712345708, 0)
	_, err = client.Model.Create().
		SetDeveloper("openai").
		SetModelID("gpt-4.1").
		SetName("GPT-4.1").
		SetType(model.TypeChat).
		SetGroup("gpt").
		SetIcon("openai").
		SetRemark(remark).
		SetModelCard(&objects.ModelCard{
			Vision:    true,
			ToolCall:  true,
			Reasoning: objects.ModelCardReasoning{Supported: true},
			Limit:     objects.ModelCardLimit{Context: 200000, Output: 8192},
			Cost:      objects.ModelCardCost{Input: 2, Output: 8, CacheRead: 0.5, CacheWrite: 1},
		}).
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type: "channel_model",
					ChannelModel: &objects.ChannelModelAssociation{
						ChannelID: ch.ID,
						ModelID:   "gpt-4.1",
					},
				},
			},
		}).
		SetStatus(model.StatusEnabled).
		SetCreatedAt(modelCreatedAt).
		Save(ctx)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4.1?include=all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var got OpenAIModel
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Equal(t, "gpt-4.1", got.ID)
	require.Equal(t, "model", got.Object)
	require.Equal(t, modelCreatedAt.Unix(), got.Created)
	require.Equal(t, "openai", got.OwnedBy)
	require.Equal(t, "GPT-4.1", got.Name)
	require.Equal(t, remark, got.Description)
	require.Equal(t, "chat", got.Type)
	require.NotNil(t, got.Capabilities)
	require.True(t, got.Capabilities.Vision)
	require.True(t, got.Capabilities.ToolCall)
	require.True(t, got.Capabilities.Reasoning)
	require.Equal(t, 200000, got.ContextLength)
	require.Equal(t, 8192, got.MaxOutputTokens)
	require.NotNil(t, got.Pricing)
	require.Equal(t, 2.0, got.Pricing.Input)
	require.Equal(t, 8.0, got.Pricing.Output)
	require.Equal(t, 0.5, got.Pricing.CacheRead)
	require.Equal(t, 1.0, got.Pricing.CacheWrite)
}

func TestOpenAIHandlers_RetrieveModel_ReturnsNotFound(t *testing.T) {
	_, _, router, _ := setupOpenAIRetrieveTest(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/models/missing-model", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)

	var got openaitypes.OpenAIError
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Equal(t, "model_not_found", got.Detail.Code)
	require.Equal(t, "invalid_request_error", got.Detail.Type)
	require.Equal(t, "model", got.Detail.Param)
	require.Contains(t, got.Detail.Message, "missing-model")
}
