//go:build integration

// 职责: 经真实鉴权、模型选择和请求持久化，对可配置的 Qwen 嵌入、重排及 AutoTrust 决策服务执行联调。
// 关联文件:
//
//	internal/server/api/openai.go
//	internal/server/api/jina.go
//	internal/server/api/typesafe.go
//	internal/server/api/responses_protocol_integration_test.go
//	internal/server/orchestrator/candidates.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	仅 AXONHUB_LIVE_PROTOCOLS=1 时访问外部服务；仅写入内存数据库，不输出凭据或请求头。
//	上游使用 EMPTY 占位凭据，仍发送 Bearer；不测试省略 Authorization。
//	AutoTrust 图像拒绝用例仅在 AXONHUB_LIVE_AUTOTRUST_IMAGE_REJECTION=1 时运行，验证文本模型的拒绝响应。
package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

type nativeLiveTarget struct {
	baseURL, model, path string
	format               llm.APIFormat
	aliases              []string
}

func TestNativeProtocolsLive(t *testing.T) {
	if os.Getenv("AXONHUB_LIVE_PROTOCOLS") != "1" {
		t.Skip("set AXONHUB_LIVE_PROTOCOLS=1 to enable external protocol integration")
	}
	readRequired := func(name string) string {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			t.Skipf("missing required configuration: %s", name)
		}
		return value
	}
	targets := []nativeLiveTarget{
		{
			baseURL: readRequired("AXONHUB_LIVE_EMBED_BASE_URL"),
			model:   readRequired("AXONHUB_LIVE_EMBED_MODEL"),
			path:    "/v2/embed",
			format:  llm.APIFormatCohereEmbed,
			aliases: []string{"live-embed-text", "live-embed-images", "live-embed-mixed"},
		},
		{
			baseURL: readRequired("AXONHUB_LIVE_RERANK_BASE_URL"),
			model:   readRequired("AXONHUB_LIVE_RERANK_MODEL"),
			path:    "/v1/rerank",
			format:  llm.APIFormatJinaRerank,
			aliases: []string{"live-rerank"},
		},
	}
	autoTrustURL := strings.TrimSpace(os.Getenv("AXONHUB_LIVE_AUTOTRUST_BASE_URL"))
	autoTrustModel := strings.TrimSpace(os.Getenv("AXONHUB_LIVE_AUTOTRUST_MODEL"))
	if autoTrustURL != "" || autoTrustModel != "" {
		require.True(t, autoTrustURL != "" && autoTrustModel != "", "AutoTrust Base URL and model must both be configured")
		targets = append(targets, nativeLiveTarget{
			baseURL: autoTrustURL,
			model:   autoTrustModel,
			path:    "/v1/decide",
			format:  llm.APIFormatAutoTrustDecide,
			aliases: []string{"live-autotrust-choice", "live-autotrust-noul", "live-autotrust-score", "live-autotrust-image-upstream-rejection"},
		})
	}
	for _, target := range targets {
		parsed, err := url.Parse(target.baseURL)
		require.True(t, err == nil, "invalid Base URL")
		require.True(t, parsed.Scheme == "http" || parsed.Scheme == "https", "Base URL must use HTTP(S)")
		require.True(t, parsed.Host != "", "Base URL must include a host")
		require.True(t, parsed.User == nil, "credentials must not be embedded in Base URL")
		require.True(t, parsed.RawQuery == "", "Base URL must not include a query")
		require.True(t, parsed.Fragment == "", "Base URL must not include a fragment")
		require.True(t, parsed.Path == "" || parsed.Path == "/", "Base URL must be the service root without /v1")
	}

	client := enttest.NewEntClient(t, "sqlite3", "file:native-protocol-live?mode=memory&_fk=1")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	setupCtx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	owner, err := client.User.Create().SetEmail("native-live@example.invalid").SetPassword("isolated-live-test-user").SetStatus(user.StatusActivated).Save(setupCtx)
	require.NoError(t, err)
	proj, err := client.Project.Create().SetName("native-live").SetStatus(project.StatusActive).Save(setupCtx)
	require.NoError(t, err)
	// 此客户端凭据只存在于独立内存库，不读取或修改用户现有的 API Key。
	const gatewayKey = "ah-native-protocol-live-isolated-client"
	_, err = client.APIKey.Create().SetName("native-live").SetKey(gatewayKey).SetUserID(owner.ID).SetProjectID(proj.ID).SetType(apikey.TypeUser).SetStatus(apikey.StatusEnabled).Save(setupCtx)
	require.NoError(t, err)
	storageRow, err := client.DataStorage.Create().SetName("native-live").SetDescription("isolated protocol integration").SetPrimary(true).SetType(datastorage.TypeDatabase).SetSettings(&objects.DataStorageSettings{}).SetStatus(datastorage.StatusActive).Save(setupCtx)
	require.NoError(t, err)
	for _, target := range targets {
		settings := &objects.ChannelSettings{}
		for _, alias := range target.aliases {
			settings.ModelMappings = append(settings.ModelMappings, objects.ModelMapping{From: alias, To: target.model})
			settings.ModelProtocols = append(settings.ModelProtocols, objects.ModelProtocol{Model: alias, APIFormats: []string{target.format.String()}})
		}
		_, err = client.Channel.Create().SetType(channel.TypeOpenai).SetName(target.aliases[0]).
			SetBaseURL(strings.TrimRight(target.baseURL, "/")).SetCredentials(objects.ChannelCredentials{APIKey: "EMPTY"}).
			SetSupportedModels([]string{target.model}).SetDefaultTestModel(target.model).SetSettings(settings).SetStatus(channel.StatusEnabled).
			SetEndpoints([]objects.ChannelEndpoint{{APIFormat: target.format.String(), Path: target.path}}).Save(setupCtx)
		require.NoError(t, err)
	}
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	system := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheConfig, Ent: client})
	require.NoError(t, system.SetDefaultDataStorageID(setupCtx, storageRow.ID))
	require.NoError(t, system.SetModelSettings(setupCtx, biz.SystemModelSettings{FallbackToChannelsOnModelNotFound: true, QueryAllChannelModels: true}))
	modelCache := biz.NewModelCacheService(biz.ModelCacheServiceParams{Ent: client})
	t.Cleanup(modelCache.Stop)
	transport := httpclient.NewHttpClient(httpclient.WithInsecureSkipVerify(true))
	channels := biz.NewChannelService(biz.ChannelServiceParams{ModelCache: modelCache, CacheConfig: cacheConfig, Ent: client, SystemService: system, HttpClient: transport})
	t.Cleanup(channels.Stop)
	usage := biz.NewUsageLogService(client, system, channels)
	storage := &biz.DataStorageService{AbstractService: &biz.AbstractService{}, SystemService: system, Cache: xcache.NewFromConfig[ent.DataStorage](cacheConfig)}
	requests := biz.NewRequestService(client, cacheConfig, system, usage, storage, biz.NewLiveStreamRegistry())
	models := biz.NewModelService(biz.ModelServiceParams{ModelCache: modelCache, ChannelService: channels, SystemService: system, Ent: client})
	selector := orchestrator.NewDefaultSelector(channels, models, system)
	protection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	t.Cleanup(protection.Stop)
	prompts := biz.NewPromptService(biz.PromptServiceParams{Ent: client})
	registry := biz.NewLiveStreamRegistry()
	limiter := orchestrator.NewChannelLimiterManager()
	openAI := NewOpenAIHandlers(OpenAIHandlersParams{
		ChannelService:              channels,
		ModelService:                models,
		DefaultSelector:             selector,
		RequestService:              requests,
		SystemService:               system,
		UsageLogService:             usage,
		PromptService:               prompts,
		PromptProtectionRuleService: protection,
		QuotaService:                biz.NewQuotaService(client, system),
		HttpClient:                  transport,
		LiveStreamRegistry:          registry,
		ChannelLimiterManager:       limiter,
		Client:                      client,
	})
	jina := NewJinaHandlers(JinaHandlersParams{
		ChannelService:              channels,
		ModelService:                models,
		DefaultSelector:             selector,
		RequestService:              requests,
		SystemService:               system,
		UsageLogService:             usage,
		PromptService:               prompts,
		PromptProtectionRuleService: protection,
		QuotaService:                biz.NewQuotaService(client, system),
		HttpClient:                  transport,
		LiveStreamRegistry:          registry,
		ChannelLimiterManager:       limiter,
	})
	autoTrust := NewTypeSafeHandlers(TypeSafeHandlersParams{
		ChannelService:              channels,
		ModelService:                models,
		DefaultSelector:             selector,
		RequestService:              requests,
		SystemService:               system,
		UsageLogService:             usage,
		PromptService:               prompts,
		PromptProtectionRuleService: protection,
		QuotaService:                biz.NewQuotaService(client, system),
		HttpClient:                  transport,
		LiveStreamRegistry:          registry,
		ChannelLimiterManager:       limiter,
	})
	projects := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
	keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client, ProjectService: projects, KeyPrefix: "ah"})
	t.Cleanup(keys.Stop)
	auth := biz.NewAuthService(biz.AuthServiceParams{APIKeyService: keys, Ent: client})
	router := gin.New()
	router.Use(middleware.WithEntClient(client), middleware.WithAPIKeyConfig(auth, nil), middleware.WithSource(request.SourceAPI))
	router.POST("/v2/embed", openAI.CreateCohereEmbedding)
	router.POST("/v1/rerank", jina.Rerank)
	router.POST("/v1/decide", autoTrust.AutoTrust)
	server := responsesIntegrationServer(t, router)
	post := func(t *testing.T, path string, body any, authorized bool, headerModel ...string) (int, []byte) {
		t.Helper()
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+path, bytes.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if len(headerModel) > 0 {
			req.Header.Set("X-AxonHub-Model", headerModel[0])
		}
		if authorized {
			req.Header.Set("Authorization", "Bearer "+gatewayKey)
		}
		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		require.NoError(t, err)
		return resp.StatusCode, data
	}
	t.Run("rejections-before-upstream", func(t *testing.T) {
		body := map[string]any{"model": targets[0].aliases[0], "texts": []string{"red square"}}
		status, _ := post(t, targets[0].path, body, false)
		require.Equal(t, http.StatusUnauthorized, status)
		body["stream"] = true
		status, _ = post(t, targets[0].path, body, true)
		require.Equal(t, http.StatusBadRequest, status)
		if len(targets) > 2 {
			body := map[string]any{"kind": "choice", "question": "Which color is the square?", "state": "The square is red.", "options": []string{"red", "blue"}}
			status, _ := post(t, targets[2].path, body, false, targets[2].aliases[0])
			require.Equal(t, http.StatusUnauthorized, status)
			body["stream"] = true
			status, _ = post(t, targets[2].path, body, true, targets[2].aliases[0])
			require.Equal(t, http.StatusBadRequest, status)
			delete(body, "stream")
			body["model"] = targets[2].aliases[0]
			status, _ = post(t, targets[2].path, body, true, "conflicting-alias")
			require.Equal(t, http.StatusBadRequest, status)
		}
		count, err := client.RequestExecution.Query().Count(setupCtx)
		require.NoError(t, err)
		require.Zero(t, count, "authentication and input rejection must precede all outbound executions")
	})
	redImage := nativeLiveRedImage(t)
	type liveCase struct {
		name        string
		target      int
		input       map[string]any
		passThrough bool
	}
	cases := []liveCase{
		{"embed-text", 0, map[string]any{"texts": []string{"a red square", "a blue circle"}, "embedding_types": []string{"float", "base64"}}, false},
		{"embed-images", 0, map[string]any{"images": []string{redImage, redImage}, "embedding_types": []string{"float"}}, true},
		{"embed-mixed", 0, map[string]any{"inputs": []any{map[string]any{"content": []any{map[string]any{"type": "text", "text": "a red square"}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": redImage}}}}}, "embedding_types": []string{"float"}}, false},
		{"rerank", 1, map[string]any{"query": "a red square", "documents": []string{"a red square", "a blue circle"}, "top_n": 2}, false},
	}
	if len(targets) > 2 {
		cases = append(cases,
			liveCase{"autotrust-choice", 2, map[string]any{"kind": "choice", "state": "The square is red.", "question": "Which color is the square?", "options": []string{"red", "blue"}}, false},
			liveCase{"autotrust-noul", 2, map[string]any{"kind": "noul", "state": "The cup arrived broken.", "question": "Is the cup damaged?"}, true},
			liveCase{"autotrust-score", 2, map[string]any{"kind": "score", "state": "The cup arrived broken.", "question": "How severe is the issue?"}, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := targets[tc.target]
			alias := "live-" + tc.name
			require.NoError(t, system.SetPassThrough(setupCtx, tc.passThrough))
			body := tc.input
			body["model"] = alias
			var headerModel []string
			if tc.target == 2 && tc.name != "autotrust-choice" {
				headerModel = []string{alias}
				if tc.name == "autotrust-noul" {
					delete(body, "model")
				}
			}
			if tc.target == 0 {
				body["output_dimension"] = 256
			}
			status, data := post(t, target.path, body, true, headerModel...)
			require.Equal(t, http.StatusOK, status, "real service call failed; response bodies and credentials are intentionally not logged")
			require.True(t, json.Valid(data))
			switch tc.target {
			case 0:
				count := 2
				if tc.name == "embed-mixed" {
					count = 1
				}
				nativeLiveAssertEmbeddings(t, data, count, tc.name == "embed-text")
			case 1:
				results := gjson.GetBytes(data, "results").Array()
				require.Len(t, results, 2)
				seen := map[int64]bool{}
				previous := math.Inf(1)
				for _, result := range results {
					require.Equal(t, gjson.Number, result.Get("index").Type)
					index := result.Get("index").Int()
					require.True(t, index == 0 || index == 1)
					require.Equal(t, tc.input["documents"].([]string)[index], result.Get("document.text").String())
					require.False(t, seen[index])
					seen[index] = true
					score := result.Get("relevance_score")
					require.Equal(t, gjson.Number, score.Type)
					require.True(t, nativeLiveFinite(score.Float()))
					require.LessOrEqual(t, score.Float(), previous)
					previous = score.Float()
				}
			case 2:
				require.Equal(t, tc.input["kind"], gjson.GetBytes(data, "kind").String())
				options := gjson.GetBytes(data, "options").Array()
				probabilities := gjson.GetBytes(data, "probabilities").Array()
				require.GreaterOrEqual(t, len(options), 2)
				require.Len(t, probabilities, len(options))
				sum := 0.0
				for index, probability := range probabilities {
					require.Equal(t, gjson.String, options[index].Type)
					require.Equal(t, gjson.Number, probability.Type)
					require.True(t, nativeLiveFinite(probability.Float()) && probability.Float() >= 0 && probability.Float() <= 1)
					sum += probability.Float()
				}
				require.InDelta(t, 1, sum, 0.001)
				choiceIndex := gjson.GetBytes(data, "choice_index")
				require.Equal(t, gjson.Number, choiceIndex.Type)
				require.True(t, choiceIndex.Int() >= 0 && choiceIndex.Int() < int64(len(options)))
				require.Equal(t, options[choiceIndex.Int()].String(), gjson.GetBytes(data, "choice").String())
				if tc.name == "autotrust-choice" {
					require.Len(t, options, len(tc.input["options"].([]string)))
					for index, option := range tc.input["options"].([]string) {
						require.Equal(t, option, options[index].String())
					}
				}
			}
			row, err := client.Request.Query().Where(request.ModelIDEQ(alias)).Only(setupCtx)
			require.NoError(t, err)
			require.Equal(t, request.StatusCompleted, row.Status)
			require.Equal(t, target.format.String(), row.Format)
			execution, err := client.RequestExecution.Query().Where(requestexecution.RequestIDEQ(row.ID)).Only(setupCtx)
			require.NoError(t, err)
			require.Equal(t, requestexecution.StatusCompleted, execution.Status)
			require.Equal(t, target.format.String(), execution.Format)
			require.Equal(t, strings.TrimRight(target.baseURL, "/")+target.path, execution.RequestURL)
			require.Equal(t, target.model, execution.ModelID)
			if tc.target == 2 {
				require.False(t, gjson.GetBytes(execution.RequestBody, "model").Exists(), "AutoTrust gateway model selector must not reach upstream")
				var headers http.Header
				require.True(t, json.Unmarshal(execution.RequestHeaders, &headers) == nil, "could not inspect saved AutoTrust headers")
				require.True(t, headers.Get("X-AxonHub-Model") == "", "AutoTrust routing header must not reach upstream")
				require.False(t, execution.PassThroughApplied, "AutoTrust adapters must validate requests and responses regardless of the pass-through setting")
				for _, field := range []string{"kind", "state", "question", "options"} {
					upstreamField := gjson.GetBytes(execution.RequestBody, field)
					if value, ok := body[field]; ok {
						expected, err := json.Marshal(value)
						require.NoError(t, err)
						require.JSONEq(t, string(expected), upstreamField.Raw)
					} else {
						require.False(t, upstreamField.Exists())
					}
				}
			} else {
				require.Equal(t, target.model, gjson.GetBytes(execution.RequestBody, "model").String())
			}
			require.JSONEq(t, string(data), string(row.ResponseBody))
			if tc.target == 0 || tc.target == 2 {
				require.JSONEq(t, string(data), string(execution.ResponseBody))
				if tc.target == 2 {
					if upstreamModel := gjson.GetBytes(data, "model"); upstreamModel.Exists() {
						require.Equal(t, upstreamModel.String(), execution.UpstreamModelID)
					}
				}
			} else {
				require.Equal(t, alias, gjson.GetBytes(data, "model").String())
				require.Equal(t, target.model, gjson.GetBytes(execution.ResponseBody, "model").String())
				results := gjson.GetBytes(data, "results").Array()
				upstreamResults := gjson.GetBytes(execution.ResponseBody, "results").Array()
				require.Len(t, upstreamResults, len(results))
				for index, result := range results {
					for _, field := range []string{"index", "relevance_score", "document.text"} {
						require.JSONEq(t, result.Get(field).Raw, upstreamResults[index].Get(field).Raw)
					}
				}
				require.JSONEq(t, gjson.GetBytes(data, "usage").Raw, gjson.GetBytes(execution.ResponseBody, "usage").Raw)
			}
			usageRow, err := client.UsageLog.Query().Where(usagelog.RequestIDEQ(row.ID)).Only(setupCtx)
			require.NoError(t, err)
			require.Positive(t, usageRow.PromptTokens+usageRow.CompletionTokens)
			require.Equal(t, execution.ChannelID, usageRow.ChannelID)
			if tc.target != 2 {
				require.Zero(t, usageRow.CompletionTokens)
				require.Equal(t, usageRow.PromptTokens, usageRow.TotalTokens)
			}
			if tc.target == 0 {
				units := gjson.GetBytes(data, "usage.billed_units")
				if !units.Get("input_tokens").Exists() && !units.Get("image_tokens").Exists() {
					units = gjson.GetBytes(data, "meta.billed_units")
				}
				require.True(t, units.Exists(), "native billed_units must be preserved")
				require.Equal(t, units.Get("input_tokens").Int()+units.Get("image_tokens").Int(), usageRow.PromptTokens)
			} else {
				require.Equal(t, gjson.GetBytes(data, "usage.prompt_tokens").Int(), usageRow.PromptTokens)
				require.Equal(t, gjson.GetBytes(data, "usage.completion_tokens").Int(), usageRow.CompletionTokens)
				require.Equal(t, gjson.GetBytes(data, "usage.total_tokens").Int(), usageRow.TotalTokens)
			}
			for _, record := range []any{row, execution, usageRow} {
				serialized, err := json.Marshal(record)
				require.True(t, err == nil, "could not inspect persisted record")
				require.False(t, strings.Contains(string(serialized), gatewayKey), "gateway credential must not appear in request, execution, or usage records")
			}
			t.Logf("status=%d format=%s upstream=%s input_tokens=%d output_tokens=%d", status, row.Format, execution.RequestURL, usageRow.PromptTokens, usageRow.CompletionTokens)
		})
	}
	if len(targets) > 2 {
		t.Run("autotrust-image-upstream-rejection", func(t *testing.T) {
			if os.Getenv("AXONHUB_LIVE_AUTOTRUST_IMAGE_REJECTION") != "1" {
				t.Skip("set AXONHUB_LIVE_AUTOTRUST_IMAGE_REJECTION=1 to test image rejection by a text-only deployment")
			}
			target := targets[2]
			const alias = "live-autotrust-image-upstream-rejection"
			require.NoError(t, system.SetPassThrough(setupCtx, false))
			body := map[string]any{"kind": "noul", "state": []any{"Examine the picture.", map[string]any{"image": redImage}}, "question": "Is the square red?"}
			status, data := post(t, target.path, body, true, alias)
			require.Equal(t, http.StatusBadRequest, status)
			require.True(t, json.Valid(data))
			require.Equal(t, target.model+" is not a multimodal model", gjson.GetBytes(data, "error.message").String())
			require.Equal(t, "BadRequestError", gjson.GetBytes(data, "error.type").String())
			require.Equal(t, gjson.Number, gjson.GetBytes(data, "error.code").Type)
			require.Equal(t, int64(http.StatusBadRequest), gjson.GetBytes(data, "error.code").Int())
			require.Equal(t, "null", gjson.GetBytes(data, "error.param").Raw)
			row, err := client.Request.Query().Where(request.ModelIDEQ(alias)).Only(setupCtx)
			require.NoError(t, err)
			execution, err := client.RequestExecution.Query().Where(requestexecution.RequestIDEQ(row.ID)).Only(setupCtx)
			require.NoError(t, err)
			require.Equal(t, request.StatusFailed, row.Status)
			require.Equal(t, requestexecution.StatusFailed, execution.Status)
			require.Equal(t, target.format.String(), row.Format)
			require.Equal(t, target.format.String(), execution.Format)
			require.Equal(t, strings.TrimRight(target.baseURL, "/")+target.path, execution.RequestURL)
			require.Equal(t, target.model, execution.ModelID)
			require.NotNil(t, execution.ResponseStatusCode)
			require.Equal(t, status, *execution.ResponseStatusCode)
			require.Equal(t, gjson.GetBytes(data, "error.message").String(), execution.ErrorMessage)
			usageCount, err := client.UsageLog.Query().Where(usagelog.RequestIDEQ(row.ID)).Count(setupCtx)
			require.NoError(t, err)
			require.Zero(t, usageCount, "rejected image requests must not produce successful usage records")
			require.Equal(t, redImage, gjson.GetBytes(execution.RequestBody, "state.1.image").String())
			require.False(t, gjson.GetBytes(execution.RequestBody, "model").Exists())
			var headers http.Header
			require.True(t, json.Unmarshal(execution.RequestHeaders, &headers) == nil, "could not inspect saved AutoTrust headers")
			require.True(t, headers.Get("X-AxonHub-Model") == "")
			for _, record := range []any{row, execution} {
				serialized, err := json.Marshal(record)
				require.True(t, err == nil, "could not inspect persisted record")
				require.False(t, strings.Contains(string(serialized), gatewayKey), "gateway credential must not appear in persisted records")
			}
			t.Logf("status=%d format=%s upstream=%s multimodal=false", status, row.Format, execution.RequestURL)
		})
	}
}

func nativeLiveRedImage(t *testing.T) string {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			picture.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, picture))
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func nativeLiveFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func nativeLiveAssertEmbeddings(t *testing.T, body []byte, count int, withBase64 bool) {
	t.Helper()
	vectors := gjson.GetBytes(body, "embeddings.float").Array()
	require.Len(t, vectors, count)
	for _, vector := range vectors {
		values := vector.Array()
		require.Len(t, values, 256)
		nonzero := false
		for _, value := range values {
			require.Equal(t, gjson.Number, value.Type)
			require.True(t, nativeLiveFinite(value.Float()))
			nonzero = nonzero || value.Float() != 0
		}
		require.True(t, nonzero)
	}
	if withBase64 {
		encoded := gjson.GetBytes(body, "embeddings.base64").Array()
		require.Len(t, encoded, count)
		for _, value := range encoded {
			decoded, err := base64.StdEncoding.DecodeString(value.String())
			require.NoError(t, err)
			require.Len(t, decoded, 256*4)
			nonzero := false
			for offset := 0; offset < len(decoded); offset += 4 {
				value := math.Float32frombits(binary.LittleEndian.Uint32(decoded[offset : offset+4]))
				require.True(t, nativeLiveFinite(float64(value)))
				nonzero = nonzero || value != 0
			}
			require.True(t, nonzero)
		}
	}
}
