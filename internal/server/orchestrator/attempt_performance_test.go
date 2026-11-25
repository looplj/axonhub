// 职责: 验证请求的最终尝试结果只计入一次渠道性能统计。
// 关联文件:
//
//	internal/server/orchestrator/performance.go
//	internal/server/orchestrator/orchestrator.go
package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPerformanceRecordsFinalAttemptOnce(t *testing.T) {
	ctx, client := setupTest(t)
	channel := createTestChannel(t, ctx, client)
	provider, err := openai.NewOutboundTransformer(channel.BaseURL, "fixture")
	require.NoError(t, err)
	selector := &staticChannelSelector{
		candidates: channelsToTestCandidates([]*biz.Channel{{Channel: channel, Outbound: provider}}, "gpt-4"),
	}
	ctx = contexts.WithProjectID(ctx, createTestProject(t, ctx, client).ID)
	executor := &mockExecutor{response: &httpclient.Response{
		StatusCode: 200, Body: buildMockOpenAIResponse("empty", "gpt-4", "", 1, 0),
	}}
	orch := newTestOrchestrator(t, selector, client, executor)
	t.Cleanup(orch.ChannelService.Stop)
	require.NoError(t, orch.SystemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled: true, EmptyResponseDetection: true, LoadBalancerStrategy: biz.LoadBalancerStrategyAdaptive,
	}))
	_, err = orch.Process(ctx, buildTestRequest("gpt-4", "hello", false))
	require.ErrorIs(t, err, pipeline.ErrEmptyResponse)

	executor.response = &httpclient.Response{StatusCode: 200, Body: buildMockOpenAIResponse("success", "gpt-4", "hello", 1, 1)}
	_, err = orch.Process(ctx, buildTestRequest("gpt-4", "hello", false))
	require.NoError(t, err)

	executor.streamEvents = []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"stream","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`)},
		{Data: []byte(`{"id":"stream","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)},
		{Data: []byte(`{"id":"stream","object":"chat.completion.chunk","model":"gpt-4","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":9,"total_tokens":10}}`)},
		&llm.DoneStreamEvent,
	}
	result, err := orch.Process(ctx, buildTestRequest("gpt-4", "hello", true))
	require.NoError(t, err)
	for result.ChatCompletionStream.Next() {
		_ = result.ChatCompletionStream.Current()
	}
	require.NoError(t, result.ChatCompletionStream.Err())
	require.NoError(t, result.ChatCompletionStream.Close())

	require.EqualValues(t, 3, executor.requestCalls.Load())
	require.Eventually(t, func() bool {
		metrics, err := orch.ChannelService.GetChannelMetrics(ctx, channel.ID)
		return err == nil && metrics.SuccessCount >= 2 && metrics.FailureCount >= 1
	}, time.Second, 10*time.Millisecond)
	metrics, err := orch.ChannelService.GetChannelMetrics(ctx, channel.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, metrics.SuccessCount)
	require.EqualValues(t, 1, metrics.FailureCount)
}
