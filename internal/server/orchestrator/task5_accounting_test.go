package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func newTask5Execution(t *testing.T, ctx context.Context, client *ent.Client, format string) (*ent.Request, *ent.RequestExecution, *PersistenceState) {
	t.Helper()
	project := createTestProject(t, ctx, client)
	channel := createTestChannel(t, ctx, client)
	requestRow, err := client.Request.Create().
		SetProjectID(project.ID).SetChannelID(channel.ID).SetModelID("gpt-5").
		SetStatus(request.StatusProcessing).SetFormat(format).SetStream(true).
		SetRequestBody([]byte(`{"stream":true}`)).Save(ctx)
	require.NoError(t, err)
	execution, err := client.RequestExecution.Create().
		SetRequestID(requestRow.ID).SetProjectID(project.ID).SetChannelID(channel.ID).
		SetModelID("gpt-5").SetFormat(format).SetStatus(requestexecution.StatusProcessing).
		SetRequestBody([]byte(`{"stream":true}`)).SetStream(true).Save(ctx)
	require.NoError(t, err)

	return requestRow, execution, &PersistenceState{}
}

func configureTask5Price(t *testing.T, ctx context.Context, client *ent.Client, channelID int) {
	t.Helper()
	input := decimal.NewFromFloat(0.01)
	output := decimal.NewFromFloat(0.02)
	_, err := client.ChannelModelPrice.Create().SetChannelID(channelID).SetModelID("gpt-5").SetPrice(objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{ItemCode: objects.PriceItemCodeUsage, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &input}},
			{ItemCode: objects.PriceItemCodeCompletion, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &output}},
		},
	}).SetReferenceID(fmt.Sprintf("task5-price-%d", channelID)).Save(ctx)
	require.NoError(t, err)
}

func TestStreamDeliveryAccounting_PersistsAuthoritativeUsageOnce(t *testing.T) {
	// Given
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	_, requestService, _, usageService := setupTestServices(t, client)
	requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIChatCompletion.String())
	_, err := client.Channel.UpdateOneID(execution.ChannelID).SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	configureTask5Price(t, ctx, client, execution.ChannelID)
	built, err := usageService.ChannelService.GetChannel(ctx, execution.ChannelID)
	require.NoError(t, err)
	usageService.ChannelService.PreloadModelPricesForTest(ctx, built)
	usageService.ChannelService.SetEnabledChannelsForTest([]*biz.Channel{built})
	transformer := &mockTransformer{
		apiFormat:          llm.APIFormatOpenAIChatCompletion,
		aggregatedResponse: []byte(`{"id":"chat-task5","choices":[{"index":0,"message":{"role":"assistant"},"finish_reason":"stop"}]}`),
		aggregatedMeta: llm.ResponseMeta{
			ID:                 "chat-task5",
			Usage:              &llm.Usage{PromptTokens: 12, CompletionTokens: 34, TotalTokens: 46},
			Completed:          true,
			AccountingEvidence: llm.AccountingEvidenceExplicitFinal,
		},
	}
	stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(`{"chunk":true}`)}}), requestRow, execution, requestService, usageService, transformer, nil, state)

	// When
	require.True(t, stream.Next())
	stream.Current()
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			errs <- stream.Close()
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	// Then
	rows, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, requestRow.ID, rows[0].RequestID)
	require.Equal(t, execution.ChannelID, rows[0].ChannelID)
	require.Equal(t, int64(12), rows[0].PromptTokens)
	require.Equal(t, int64(34), rows[0].CompletionTokens)
	require.NotNil(t, rows[0].TotalCost)
	require.Positive(t, *rows[0].TotalCost)
	require.Equal(t, requestexecution.StatusCompleted, mustExecution(t, ctx, client, execution.ID).Status)
}

func TestStreamDeliveryAccounting_ResponseFailedTerminalRetainsUsage(t *testing.T) {
	// Given
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	_, requestService, _, usageService := setupTestServices(t, client)
	requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIResponse.String())
	outbound, err := responses.NewOutboundTransformer("https://example.test", "synthetic-key")
	require.NoError(t, err)
	event := &httpclient.StreamEvent{Type: "response.failed", Data: []byte(`{"type":"response.failed","response":{"id":"resp-failed","status":"failed","usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}}}`)}
	stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{event}), requestRow, execution, requestService, usageService, outbound, nil, state)

	// When
	require.True(t, stream.Next())
	stream.Current()
	require.NoError(t, stream.Close())

	// Then
	saved, err := client.RequestExecution.Get(ctx, execution.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusFailed, saved.Status)
	rows, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(46), rows[0].TotalTokens)
}

func TestStreamDeliveryAccounting_UnsuccessfulResponsesPersistUsageMatrix(t *testing.T) {
	for _, terminal := range []struct {
		name      string
		eventType string
		status    requestexecution.Status
	}{
		{name: "failed", eventType: "response.failed", status: requestexecution.StatusFailed},
		{name: "incomplete", eventType: "response.incomplete", status: requestexecution.StatusFailed},
		{name: "cancelled", eventType: "response.cancelled", status: requestexecution.StatusCanceled},
	} {
		for _, usage := range []struct {
			name     string
			input    int64
			output   int64
			wantCost bool
		}{
			{name: "nonzero", input: 12, output: 34, wantCost: true},
			{name: "explicit zero"},
		} {
			t.Run(terminal.name+"/"+usage.name, func(t *testing.T) {
				client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
				t.Cleanup(func() { require.NoError(t, client.Close()) })
				ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
				_, requestService, _, usageService := setupTestServices(t, client)
				requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIResponse.String())
				_, err := client.Channel.UpdateOneID(execution.ChannelID).SetStatus(channel.StatusEnabled).Save(ctx)
				require.NoError(t, err)
				configureTask5Price(t, ctx, client, execution.ChannelID)
				built, err := usageService.ChannelService.GetChannel(ctx, execution.ChannelID)
				require.NoError(t, err)
				usageService.ChannelService.PreloadModelPricesForTest(ctx, built)
				usageService.ChannelService.SetEnabledChannelsForTest([]*biz.Channel{built})
				event := &httpclient.StreamEvent{Type: terminal.eventType, Data: []byte(fmt.Sprintf(
					`{"type":%q,"response":{"id":"%s","status":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"total_tokens":%d}}}`,
					terminal.eventType, terminal.name, terminal.eventType[len("response."):], usage.input, usage.output, usage.input+usage.output,
				))}
				outbound, err := responses.NewOutboundTransformer("https://example.test", "synthetic-key")
				require.NoError(t, err)
				stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{event}), requestRow, execution, requestService, usageService, outbound, nil, state)
				require.True(t, stream.Next())
				stream.Current()
				require.NoError(t, stream.Close())
				require.NoError(t, stream.Close())

				saved, err := client.RequestExecution.Get(ctx, execution.ID)
				require.NoError(t, err)
				require.Equal(t, terminal.status, saved.Status)
				rows, err := client.UsageLog.Query().All(ctx)
				require.NoError(t, err)
				require.Len(t, rows, 1)
				require.Equal(t, requestRow.ID, rows[0].RequestID)
				require.Equal(t, execution.ProjectID, rows[0].ProjectID)
				require.Equal(t, execution.ChannelID, rows[0].ChannelID)
				require.Equal(t, execution.ModelID, rows[0].ModelID)
				require.Equal(t, usage.input, rows[0].PromptTokens)
				require.Equal(t, usage.output, rows[0].CompletionTokens)
				require.Equal(t, usage.input+usage.output, rows[0].TotalTokens)
				require.Equal(t, usagelog.Source(requestRow.Source), rows[0].Source)
				require.Equal(t, requestRow.Format, rows[0].Format)
				if usage.wantCost {
					require.NotNil(t, rows[0].TotalCost)
					require.Positive(t, *rows[0].TotalCost)
				} else {
					require.NotNil(t, rows[0].TotalCost)
					require.Zero(t, *rows[0].TotalCost)
				}
			})
		}
	}
}

func TestStreamDeliveryAccounting_ExplicitZeroAndAmbiguousUsage(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		evidence llm.AccountingEvidence
		wantRows int
	}{
		{name: "explicit final zero", evidence: llm.AccountingEvidenceExplicitFinal, wantRows: 1},
		{name: "ambiguous usage is unavailable", evidence: llm.AccountingEvidenceUnavailable, wantRows: 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			_, requestService, _, usageService := setupTestServices(t, client)
			requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIChatCompletion.String())
			transformer := &mockTransformer{
				apiFormat:          llm.APIFormatOpenAIChatCompletion,
				aggregatedResponse: []byte(`{"id":"chat-zero","choices":[{"index":0,"message":{"role":"assistant"},"finish_reason":"stop"}]}`),
				aggregatedMeta: llm.ResponseMeta{
					ID:                 "chat-zero",
					Usage:              &llm.Usage{},
					Completed:          true,
					AccountingEvidence: scenario.evidence,
				},
			}
			stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(`{"chunk":true}`)}}), requestRow, execution, requestService, usageService, transformer, nil, state)

			// When
			require.True(t, stream.Next())
			stream.Current()
			require.NoError(t, stream.Close())

			// Then
			rows, err := client.UsageLog.Query().All(ctx)
			require.NoError(t, err)
			require.Len(t, rows, scenario.wantRows)
		})
	}
}

func TestStreamDeliveryAccounting_RealAggregatorsPersistTerminalUsage(t *testing.T) {
	tests := []struct {
		name   string
		format llm.APIFormat
		out    func() transformer.Outbound
		events func(int64, int64) []*httpclient.StreamEvent
	}{
		{
			name: "chat choices-less final", format: llm.APIFormatOpenAIChatCompletion,
			out: func() transformer.Outbound {
				out, err := openai.NewOutboundTransformer("https://example.test", "key")
				require.NoError(t, err)
				return out
			},
			events: func(input, output int64) []*httpclient.StreamEvent {
				return []*httpclient.StreamEvent{
					{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`)},
					{Data: []byte(fmt.Sprintf(`{"id":"chat","model":"gpt-5","choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`, input, output, input+output))},
				}
			},
		},
		{
			name: "responses failed terminal", format: llm.APIFormatOpenAIResponse,
			out: func() transformer.Outbound {
				out, err := responses.NewOutboundTransformer("https://example.test", "key")
				require.NoError(t, err)
				return out
			},
			events: func(input, output int64) []*httpclient.StreamEvent {
				return []*httpclient.StreamEvent{{Data: []byte(fmt.Sprintf(`{"type":"response.failed","response":{"id":"resp","status":"failed","usage":{"input_tokens":%d,"output_tokens":%d,"total_tokens":%d}}}`, input, output, input+output))}}
			},
		},
		{
			name: "anthropic message stop", format: llm.APIFormatAnthropicMessage,
			out: func() transformer.Outbound {
				out, err := anthropic.NewOutboundTransformer("https://example.test", "key")
				require.NoError(t, err)
				return out
			},
			events: func(input, output int64) []*httpclient.StreamEvent {
				return []*httpclient.StreamEvent{
					{Data: []byte(fmt.Sprintf(`{"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","content":[],"model":"claude","usage":{"input_tokens":%d,"output_tokens":0}}}`, input))},
					{Data: []byte(fmt.Sprintf(`{"type":"message_delta","usage":{"input_tokens":%d,"output_tokens":%d},"delta":{"stop_reason":"end_turn"}}`, input, output))},
					{Data: []byte(`{"type":"message_stop"}`)},
				}
			},
		},
	}
	for _, tt := range tests {
		for _, usage := range []struct {
			name          string
			input, output int64
		}{{name: "nonzero", input: 12, output: 34}, {name: "zero"}} {
			t.Run(tt.name+"/"+usage.name, func(t *testing.T) {
				client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
				t.Cleanup(func() { require.NoError(t, client.Close()) })
				ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
				_, requestService, _, usageService := setupTestServices(t, client)
				requestRow, execution, state := newTask5Execution(t, ctx, client, tt.format.String())
				_, err := client.Channel.UpdateOneID(execution.ChannelID).SetStatus(channel.StatusEnabled).Save(ctx)
				require.NoError(t, err)
				configureTask5Price(t, ctx, client, execution.ChannelID)
				built, err := usageService.ChannelService.GetChannel(ctx, execution.ChannelID)
				require.NoError(t, err)
				usageService.ChannelService.PreloadModelPricesForTest(ctx, built)
				usageService.ChannelService.SetEnabledChannelsForTest([]*biz.Channel{built})
				stream := NewOutboundPersistentStream(ctx, streams.SliceStream(tt.events(usage.input, usage.output)), requestRow, execution, requestService, usageService, tt.out(), nil, state)
				for stream.Next() {
					stream.Current()
				}
				require.NoError(t, stream.Close())
				rows, err := client.UsageLog.Query().All(ctx)
				require.NoError(t, err)
				require.Len(t, rows, 1)
				require.Equal(t, requestRow.ID, rows[0].RequestID)
				require.Equal(t, usage.input+usage.output, rows[0].TotalTokens)
				if usage.input+usage.output > 0 {
					require.NotNil(t, rows[0].TotalCost)
				}
			})
		}
	}
}

func TestStreamDeliveryAccounting_ChatCleanEOFAndAmbiguousPlaceholders(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		events     []*httpclient.StreamEvent
		streamErr  error
		wantRows   int
		wantReason string
	}{
		{
			name:     "finish-only nonzero clean EOF",
			events:   []*httpclient.StreamEvent{{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`)}},
			wantRows: 1,
		},
		{
			name:     "finish-only zero clean EOF",
			events:   []*httpclient.StreamEvent{{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)}},
			wantRows: 1,
		},
		{
			name: "repeated zero truncation",
			events: []*httpclient.StreamEvent{
				{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)},
				{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)},
			},
			streamErr:  errors.New("provider stream interrupted"),
			wantReason: "provider-error",
		},
		{
			name: "repeated zero clean EOF",
			events: []*httpclient.StreamEvent{
				{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)},
				{Data: []byte(`{"id":"chat","model":"gpt-5","choices":[{"index":0,"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)},
			},
			wantReason: "absent",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			_, requestService, _, usageService := setupTestServices(t, client)
			requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIChatCompletion.String())
			outbound, err := openai.NewOutboundTransformer("https://example.test", "key")
			require.NoError(t, err)
			stream := NewOutboundPersistentStream(ctx, &task5ErrorStream{events: scenario.events, err: scenario.streamErr}, requestRow, execution, requestService, usageService, outbound, nil, state)
			for stream.Next() {
				stream.Current()
			}
			require.NoError(t, stream.Close())
			rows, err := client.UsageLog.Query().All(ctx)
			require.NoError(t, err)
			require.Len(t, rows, scenario.wantRows)
			if scenario.wantReason != "" {
				require.Equal(t, scenario.wantReason, accountingUnavailableReason(
					llm.ResponseMeta{AccountingEvidence: llm.AccountingEvidenceUnavailable},
					func() llm.SourceEndReason {
						if scenario.streamErr != nil {
							return llm.SourceEndError
						}
						return llm.SourceEndCleanEOF
					}(),
				))
			}
		})
	}
}

type task5ErrorStream struct {
	events []*httpclient.StreamEvent
	err    error
	index  int
}

func (s *task5ErrorStream) Next() bool {
	return s.index < len(s.events)
}

func (s *task5ErrorStream) Current() *httpclient.StreamEvent {
	event := s.events[s.index]
	s.index++
	return event
}

func (s *task5ErrorStream) Err() error {
	if s.index == len(s.events) {
		return s.err
	}
	return nil
}

func (s *task5ErrorStream) Close() error { return nil }

func TestStreamDeliveryAccounting_AttributionAndQuota(t *testing.T) {
	// Given: two independently attributed executions in the same database.
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	_, requestService, systemService, usageService := setupTestServices(t, client)
	quota := biz.NewQuotaService(client, systemService)
	hooks := 0
	usageService.OnUsageLogCreated = func() { hooks++ }
	for i, counts := range []struct {
		input, output, cached, reasoning int64
		cost                             float64
	}{
		{input: 12, output: 34, cached: 2, reasoning: 3, cost: 0.00000078},
		{},
	} {
		project, err := client.Project.Create().SetName(fmt.Sprintf("task5-project-%d", i)).Save(ctx)
		require.NoError(t, err)
		apiKey, err := client.APIKey.Create().SetProjectID(project.ID).SetKey(fmt.Sprintf("task5-attribution-key-%d", i)).SetName("task5 attribution").Save(ctx)
		require.NoError(t, err)
		channelRow, err := client.Channel.Create().SetType(channel.TypeOpenai).SetName(fmt.Sprintf("task5-channel-%d", i)).SetBaseURL("https://example.test").SetCredentials(objects.ChannelCredentials{APIKey: fmt.Sprintf("provider-key-%d", i)}).SetSupportedModels([]string{"gpt-5"}).SetDefaultTestModel("gpt-5").SetStatus(channel.StatusEnabled).Save(ctx)
		require.NoError(t, err)
		requestRow, err := client.Request.Create().SetAPIKeyID(apiKey.ID).SetProjectID(project.ID).SetChannelID(channelRow.ID).SetModelID("client-model").SetSource(request.SourceTest).SetStatus(request.StatusProcessing).SetFormat(llm.APIFormatOpenAIChatCompletion.String()).SetStream(true).SetRequestBody([]byte(`{"stream":true}`)).Save(ctx)
		require.NoError(t, err)
		execution, err := client.RequestExecution.Create().SetRequestID(requestRow.ID).SetProjectID(project.ID).SetChannelID(channelRow.ID).SetModelID("gpt-5").SetFormat(requestRow.Format).SetStatus(requestexecution.StatusProcessing).SetRequestBody([]byte(`{"stream":true}`)).SetStream(true).Save(ctx)
		require.NoError(t, err)
		configureTask5Price(t, ctx, client, channelRow.ID)
		built, err := usageService.ChannelService.GetChannel(ctx, channelRow.ID)
		require.NoError(t, err)
		usageService.ChannelService.PreloadModelPricesForTest(ctx, built)
		usageService.ChannelService.SetEnabledChannelsForTest([]*biz.Channel{built})
		outbound, err := openai.NewOutboundTransformer("https://example.test", "key")
		require.NoError(t, err)
		state := &PersistenceState{}
		stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{
			{Data: []byte(`{"id":"chat","model":"provider-wire-model","choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`)},
			{Data: []byte(fmt.Sprintf(`{"id":"chat","model":"provider-wire-model","choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d,"prompt_tokens_details":{"cached_tokens":%d},"completion_tokens_details":{"reasoning_tokens":%d}}}`, counts.input, counts.output, counts.input+counts.output, counts.cached, counts.reasoning))},
		}), requestRow, execution, requestService, usageService, outbound, nil, state)

		// When: final usage is consumed and the same owner closes repeatedly.
		for stream.Next() {
			stream.Current()
		}
		require.NoError(t, stream.Close())
		require.NoError(t, stream.Close())

		// Then: attribution, pricing, quota aggregation, and hook remain distinct.
		row, err := client.UsageLog.Query().Where(usagelog.RequestIDEQ(requestRow.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, requestRow.ID, row.RequestID)
		require.Equal(t, apiKey.ID, row.APIKeyID)
		require.Equal(t, project.ID, row.ProjectID)
		require.Equal(t, channelRow.ID, row.ChannelID)
		require.Equal(t, "gpt-5", row.ModelID)
		require.Equal(t, requestRow.Format, row.Format)
		require.Equal(t, usagelog.SourceTest, row.Source)
		require.Equal(t, counts.input, row.PromptTokens)
		require.Equal(t, counts.output, row.CompletionTokens)
		require.Equal(t, counts.input+counts.output, row.TotalTokens)
		require.Equal(t, counts.cached, row.PromptCachedTokens)
		require.Equal(t, counts.reasoning, row.CompletionReasoningTokens)
		require.Equal(t, fmt.Sprintf("task5-price-%d", channelRow.ID), row.CostPriceReferenceID)
		require.NotNil(t, row.TotalCost)
		require.InDelta(t, counts.cost, *row.TotalCost, 1e-15)
		saved := mustExecution(t, ctx, client, execution.ID)
		require.Equal(t, requestexecution.StatusCompleted, saved.Status)
		require.Equal(t, requestRow.ID, saved.RequestID)
		require.Equal(t, project.ID, saved.ProjectID)
		require.Equal(t, channelRow.ID, saved.ChannelID)
		require.Equal(t, "gpt-5", saved.ModelID)
		require.Equal(t, "provider-wire-model", saved.UpstreamModelID)
		require.Equal(t, requestRow.Format, saved.Format)
		result, err := quota.GetQuota(ctx, apiKey.ID, &objects.APIKeyQuota{Period: objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime}})
		require.NoError(t, err)
		require.Equal(t, int64(1), result.Usage.RequestCount)
		require.Equal(t, counts.input+counts.output, result.Usage.TotalTokens)
		require.InDelta(t, counts.cost, result.Usage.TotalCost.InexactFloat64(), 1e-15)
		require.Equal(t, i+1, hooks)
	}
	rows, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NotEqual(t, rows[0].RequestID, rows[1].RequestID)
	require.NotEqual(t, rows[0].ProjectID, rows[1].ProjectID)
	require.NotEqual(t, rows[0].APIKeyID, rows[1].APIKeyID)
	require.NotEqual(t, rows[0].ChannelID, rows[1].ChannelID)
}

func TestStreamDeliveryAccounting_PersistenceErrorIsReturned(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	_, requestService, _, usageService := setupTestServices(t, client)
	requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIChatCompletion.String())
	streamTransformer := &mockTransformer{
		apiFormat:          llm.APIFormatOpenAIChatCompletion,
		aggregatedResponse: []byte(`{"id":"persist-error","choices":[{"index":0,"message":{"role":"assistant"},"finish_reason":"stop"}]}`),
		aggregatedMeta: llm.ResponseMeta{
			ID:                 "persist-error",
			Usage:              &llm.Usage{PromptTokens: 12, CompletionTokens: 34, TotalTokens: 46},
			Completed:          true,
			AccountingEvidence: llm.AccountingEvidenceExplicitFinal,
		},
	}
	stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(`{"chunk":true}`)}}), requestRow, execution, requestService, usageService, streamTransformer, nil, state)
	require.True(t, stream.Next())
	stream.Current()
	require.NoError(t, client.Close())
	require.Error(t, stream.Close())
}

func TestStreamDeliveryAccounting_UnavailableReasons(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		meta      llm.ResponseMeta
		sourceEnd llm.SourceEndReason
		want      string
	}{
		{name: "absent", meta: llm.ResponseMeta{}, sourceEnd: llm.SourceEndCleanEOF, want: "absent"},
		{name: "truncated", meta: llm.ResponseMeta{AccountingEvidence: llm.AccountingEvidenceCleanEndCandidate}, sourceEnd: llm.SourceEndCanceled, want: "truncated"},
		{name: "provider error", meta: llm.ResponseMeta{}, sourceEnd: llm.SourceEndError, want: "provider-error"},
		{name: "budget", meta: llm.ResponseMeta{Usage: &llm.Usage{}}, sourceEnd: llm.SourceEndBudget, want: "budget"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			require.Equal(t, scenario.want, accountingUnavailableReason(scenario.meta, scenario.sourceEnd))
		})
	}
}

func TestStreamDeliveryAccounting_PreservesBudgetSourceEnd(t *testing.T) {
	if got := (&OutboundPersistentStream{}).accountingSourceEnd(nil, nil); got != llm.SourceEndUnknown {
		t.Fatalf("zero stream end = %q, want unknown", got)
	}
	stream := &OutboundPersistentStream{sourceEnd: llm.SourceEndBudget}
	if got := stream.accountingSourceEnd(nil, nil); got != llm.SourceEndBudget {
		t.Fatalf("budget stream end = %q, want budget", got)
	}
	meta := llm.ResponseMeta{Usage: &llm.Usage{}, AccountingEvidence: llm.AccountingEvidenceCleanEndCandidate}
	if accountingMayBeFinal(llm.APIFormatOpenAIChatCompletion, meta, llm.SourceEndBudget) {
		t.Fatal("budget source end fabricated final accounting from zero usage")
	}
	if got := accountingUnavailableReason(meta, llm.SourceEndBudget); got != "budget" {
		t.Fatalf("budget accounting reason = %q, want budget", got)
	}
}

func TestStreamDeliveryAccounting_CloseDefersUntilSourceOwnerReleases(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	_, requestService, _, usageService := setupTestServices(t, client)
	requestRow, execution, state := newTask5Execution(t, ctx, client, llm.APIFormatOpenAIChatCompletion.String())
	transformer := &mockTransformer{
		apiFormat:          llm.APIFormatOpenAIChatCompletion,
		aggregatedResponse: []byte(`{"id":"held","choices":[{"index":0,"message":{"role":"assistant"},"finish_reason":"stop"}]}`),
		aggregatedMeta: llm.ResponseMeta{
			ID:                 "held",
			Usage:              &llm.Usage{PromptTokens: 12, CompletionTokens: 34, TotalTokens: 46},
			Completed:          true,
			AccountingEvidence: llm.AccountingEvidenceExplicitFinal,
		},
	}
	stream := NewOutboundPersistentStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(`{"chunk":true}`)}}), requestRow, execution, requestService, usageService, transformer, nil, state)
	stream.HoldFinalization()
	require.True(t, stream.Next())
	stream.Current()

	// The consumer can close without waiting for the source owner.
	require.NoError(t, stream.Close())
	rows, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Empty(t, rows)

	// The source owner releases the barrier after reader/consumer quiescence.
	require.NoError(t, stream.FinalizeAccounting(llm.SourceEndSemantic))
	rows, err = client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func mustExecution(t *testing.T, ctx context.Context, client *ent.Client, id int) *ent.RequestExecution {
	t.Helper()
	execution, err := client.RequestExecution.Get(ctx, id)
	require.NoError(t, err)
	return execution
}
