// 职责: 验证实时模型权限、两级映射、渠道权限与供应商协议隔离。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/orchestrator/candidates.go

package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

func TestRealtimeModelAuthorizationAndMapping(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("route-project").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	actual := "gpt-realtime-2.1-mini"
	row := db.Channel.Create().SetType(channel.TypeOpenai).SetName("route-channel").SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-upstream"}).SetEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "/custom/chat"}}).SetSupportedModels([]string{actual, openairt.TranslationModel}).SetDefaultTestModel(actual).SetStatus(channel.StatusEnabled).SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime", HandshakeTimeoutMs: lo.ToPtr(int64(30000))}, ModelProtocols: []objects.ModelProtocol{{Model: actual, APIFormats: []string{"openai/chat_completions"}, Enabled: lo.ToPtr(true)}}}).SaveX(ctx)
	price := objects.ModelPrice{}
	for _, code := range objects.AllRealtimePriceItemCodes() {
		rate := decimal.NewFromInt(1)
		price.Items = append(price.Items, objects.ModelPriceItem{ItemCode: code, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate}})
	}
	db.ChannelModelPrice.Create().SetChannelID(row.ID).SetModelID(actual).SetReferenceID("route-price").SetPrice(price).SaveX(ctx)
	db.Model.Create().SetModelID("route-model").SetName("route-model").SetDeveloper("openai").SetIcon("openai").SetGroup("openai").SetModelCard(&objects.ModelCard{}).SetType("chat").SetStatus("enabled").SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{{Type: "channel_model", ChannelModel: &objects.ChannelModelAssociation{ChannelID: row.ID, ModelID: actual}}}}).SaveX(ctx)
	key := db.APIKey.Create().SetName("route-key").SetKey("synthetic-client").SetProjectID(project.ID).SetType(apikey.TypeUser).SetScopes([]string{"read_channels", "write_requests"}).SetProfiles(&objects.APIKeyProfiles{ActiveProfile: "restricted", Profiles: []objects.APIKeyProfile{{Name: "restricted", ModelIDs: []string{"voice"}, ModelMappings: []objects.ModelMapping{{From: "voice", To: "route-model"}}, ChannelIDs: []int{row.ID}}}}).SaveX(ctx)
	key.Edges.Project = project
	channels := newTestChannelServiceForChannels(db)
	built, err := channels.GetChannel(ctx, row.ID)
	require.NoError(t, err)
	channels.PreloadModelPricesForTest(ctx, built)
	channels.SetEnabledChannelsForTest([]*biz.Channel{built})
	system := newTestSystemService(db)
	service := &RealtimeService{Channels: channels, Selector: NewDefaultSelector(channels, newTestModelService(db), system), Usage: biz.NewUsageLogService(db, system, channels)}
	_, err = service.Prepare(context.Background(), "voice", false)
	require.Error(t, err)
	ctx = contexts.WithAPIKey(ctx, key)
	_, err = service.Prepare(ctx, "unauthorized", false)
	require.ErrorIs(t, err, biz.ErrInvalidModel)
	target, err := service.Prepare(ctx, "voice", false)
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, target.HandshakeTimeout)
	require.Equal(t, actual, target.Model)
	require.Equal(t, "voice", target.ClientModel)
	require.Equal(t, llm.APIFormatOpenAIRealtimeWebSocket, target.APIFormat)
	require.Equal(t, "wss://api.openai.com/v1/realtime?model="+actual, target.URL)
	require.NotContains(t, target.SafetyIdentifier, key.Key)
	require.Len(t, target.SafetyIdentifier, 64)
	chatCandidates, err := service.Selector.Select(ctx, &llm.Request{Model: actual, RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatOpenAIChatCompletion})
	require.NoError(t, err)
	require.Empty(t, chatCandidates, "Realtime selection excludes the channel from ordinary routing")
	require.Len(t, built.ResolveEndpoints(), 1)
	require.Equal(t, llm.APIFormatOpenAIRealtimeWebSocket.String(), built.ResolveEndpoints()[0].APIFormat)
	require.Equal(t, "/custom/chat", db.Channel.GetX(ctx, row.ID).Endpoints[0].Path)

	built.Settings.Realtime.Enabled = false
	chatCandidates, err = service.Selector.selectChannelCadidates(ctx, &llm.Request{Model: actual, RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatOpenAIChatCompletion})
	require.NoError(t, err)
	require.Len(t, chatCandidates, 1)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), chatCandidates[0].APIFormat)
	require.Contains(t, built.ResolveEndpoints(), objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "/custom/chat"})

	built.Settings.Realtime.Enabled = true
	_, err = service.PrepareTranslation(ctx, "voice")
	require.ErrorIs(t, err, biz.ErrInvalidModel, "translation route excludes ordinary realtime models")
	httpRequest := &llm.Request{Model: openairt.TranslationModel, RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatOpenAIChatCompletion}
	translationChat, err := service.Selector.selectChannelCadidates(ctx, httpRequest)
	require.NoError(t, err)
	require.Empty(t, translationChat, "ordinary HTTP cannot select the translation model even when the channel has built-in chat endpoints")
	require.Empty(t, SelectAPIFormat([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIRealtimeWebSocket.String()}}, httpRequest))

	_, err = service.Prepare(ctx, "voice", true)
	require.Error(t, err, "native Qwen entry must not select official OpenAI")
	key.Profiles.Profiles[0].ChannelIDs = []int{999}
	_, err = service.Prepare(ctx, "voice", false)
	require.Error(t, err, "channel permissions cannot be bypassed")
	key.Profiles.Profiles[0].ChannelIDs = nil
	broken := db.Channel.Create().SetType(channel.TypeOpenai).SetName("incomplete-price-channel").SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SetSupportedModels([]string{actual}).SetDefaultTestModel(actual).SetStatus(channel.StatusEnabled).SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}).SaveX(ctx)
	brokenBuilt, err := channels.GetChannel(ctx, broken.ID)
	require.NoError(t, err)
	channels.SetEnabledChannelsForTest([]*biz.Channel{brokenBuilt, built})
	library := db.Model.Query().OnlyX(ctx)
	db.Model.UpdateOne(library).SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{{Type: "channel_model", Priority: 0, ChannelModel: &objects.ChannelModelAssociation{ChannelID: broken.ID, ModelID: actual}}, {Type: "channel_model", Priority: 10, ChannelModel: &objects.ChannelModelAssociation{ChannelID: row.ID, ModelID: actual}}}}).SaveX(ctx)
	service.Selector = NewDefaultSelector(channels, newTestModelService(db), system)
	target, err = service.Prepare(ctx, "voice", false)
	require.NoError(t, err)
	require.Equal(t, broken.ID, target.Channel.ID, "a channel with no price keeps its configured routing priority")
	require.Nil(t, target.Price)

	service.Requests = newTestRequestServiceForChannels(db, system)
	audit, err := service.Requests.CreateRealtimeRequest(ctx, target.ClientModel, target.Channel.ID, "unpriced-openai-session", objects.JSONRawMessage("{}"))
	require.NoError(t, err)
	usageLog, err := service.Usage.CreateRealtimeUsage(ctx, biz.CreateUsageLogParams{
		RequestID: audit.ID, ProjectID: project.ID, ChannelID: target.Channel.ID, ActualModelID: target.Model,
		Usage: &llm.Usage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18}, Source: usagelog.SourceAPI,
		Format: string(llm.APIFormatOpenAIRealtime), APIKeyID: lo.ToPtr(key.ID),
	}, target.Price)
	require.NoError(t, err)
	require.EqualValues(t, 18, usageLog.TotalTokens)
	require.Nil(t, usageLog.TotalCost)
	require.Empty(t, usageLog.CostItems)
}

func TestRealtimeReservedRequestDoesNotCancelItsOwnQuotaSlot(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("request-quota").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	key := db.APIKey.Create().SetName("quota").SetKey("synthetic-quota").SetProjectID(project.ID).SetType("user").SetProfiles(&objects.APIKeyProfiles{ActiveProfile: "one", Profiles: []objects.APIKeyProfile{{Name: "one", Quota: &objects.APIKeyQuota{Requests: lo.ToPtr(int64(1)), Period: objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime}}}}}).SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)
	system := newTestSystemService(db)
	service := &RealtimeService{Quota: biz.NewQuotaService(db, system)}
	require.NoError(t, service.checkQuota(ctx, key))
	requests := newTestRequestServiceForChannels(db, system)
	record, err := requests.CreateRealtimeRequest(ctx, "voice", 1, "rt-session-quota", objects.JSONRawMessage(`{}`))
	require.NoError(t, err)
	require.NoError(t, service.checkQuota(ctx, key, false), "runtime checks must allow the already admitted request to finish")
	require.NoError(t, service.checkQuota(ctx, key), "existing request quotas count confirmed usage, not in-flight metadata")
	db.UsageLog.Create().SetRequestID(record.ID).SetProjectID(project.ID).SetAPIKeyID(key.ID).SetChannelID(1).SetModelID("voice").SetPromptTokens(1).SetCompletionTokens(1).SetTotalTokens(2).SetFormat("openai/realtime").SetSource("api").SaveX(ctx)
	require.NoError(t, service.checkQuota(ctx, key, false))
	require.ErrorIs(t, service.checkQuota(ctx, key), rt.ErrQuotaExceeded, "the next reservation after confirmed usage must be refused")
}

func TestUnpricedQwenRealtimeChannelCanPrepareAndRecordUsage(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("unpriced-qwen-realtime").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	model := "qwen3.8-omni-flash-realtime"
	key := db.APIKey.Create().SetName("unpriced-qwen-realtime").SetKey("synthetic-unpriced-qwen").SetProjectID(project.ID).SetType("user").SetScopes([]string{"read_channels", "write_requests"}).SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)
	channelRow := db.Channel.Create().SetType(channel.TypeBailian).SetName("unpriced-qwen-realtime").SetBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-upstream"}).SetSupportedModels([]string{model}).SetDefaultTestModel(model).SetStatus("enabled").SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "qwen/realtime-ws", BaseURL: "wss://ws.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}}).SaveX(ctx)
	channels := newTestChannelServiceForChannels(db)
	built, err := channels.GetChannel(ctx, channelRow.ID)
	require.NoError(t, err)
	channels.SetEnabledChannelsForTest([]*biz.Channel{built})
	system := newTestSystemService(db)
	requests := newTestRequestServiceForChannels(db, system)
	usage := biz.NewUsageLogService(db, system, channels)
	alias := "qwen-unpriced-alias"
	db.Model.Create().SetModelID(alias).SetName(alias).SetDeveloper("qwen").SetIcon("qwen").SetGroup("qwen").SetModelCard(&objects.ModelCard{}).SetType("chat").SetStatus("enabled").SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{{Type: "channel_model", ChannelModel: &objects.ChannelModelAssociation{ChannelID: channelRow.ID, ModelID: model}}}}).SaveX(ctx)
	service := &RealtimeService{
		Channels: channels,
		Selector: NewDefaultSelector(channels, newTestModelService(db), newTestSystemService(db)),
		Usage:    usage, Requests: requests,
	}

	target, err := service.Prepare(ctx, alias, false)
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatQwenRealtimeWebSocket, target.APIFormat)
	require.Nil(t, target.Price)

	audit, err := requests.CreateRealtimeRequest(ctx, model, channelRow.ID, "unpriced-qwen-session", objects.JSONRawMessage("{}"))
	require.NoError(t, err)
	row, err := usage.CreateRealtimeUsage(ctx, biz.CreateUsageLogParams{
		RequestID: audit.ID, ProjectID: project.ID, ChannelID: channelRow.ID, ActualModelID: model,
		Usage:  &llm.Usage{PromptTokens: 17, CompletionTokens: 5, TotalTokens: 22},
		Source: usagelog.SourceAPI, Format: string(llm.APIFormatOpenAIRealtime), APIKeyID: &key.ID,
	}, target.Price)
	require.NoError(t, err)
	require.EqualValues(t, 22, row.TotalTokens)
	require.Nil(t, row.TotalCost)
	require.Empty(t, row.CostItems)
}
