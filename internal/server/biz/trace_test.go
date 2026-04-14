package biz

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xfile"
)

func setupTestTraceService(t *testing.T, client *ent.Client) (*TraceService, *ent.Client) {
	t.Helper()

	if client == nil {
		client = enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	}

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	_, err := client.DataStorage.Query().
		Where(datastorage.PrimaryEQ(true)).
		First(ctx)
	if ent.IsNotFound(err) {
		createTestDataStorage(t, client, ctx, "primary-storage", true, datastorage.TypeDatabase)
	} else {
		require.NoError(t, err)
	}

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	dataStorageService := NewDataStorageService(
		DataStorageServiceParams{
			SystemService: systemService,
			CacheConfig:   xcache.Config{},
			Executor:      executors.NewPoolScheduleExecutor(),
			Client:        client,
		},
	)
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	traceService := NewTraceService(TraceServiceParams{
		RequestService: NewRequestService(client, systemService, usageLogService, dataStorageService, NewLiveStreamRegistry()),
		Ent:            client,
	})

	return traceService, client
}

func findSpanByType(spans []Span, spanType string) *Span {
	for i := range spans {
		if spans[i].Type == spanType {
			return &spans[i]
		}
	}

	return nil
}

func countSpansByType(spans []Span, spanType string) int {
	count := 0

	for _, span := range spans {
		if span.Type == spanType {
			count++
		}
	}

	return count
}

func TestRequestService_LoadersReturnEmptyJSONAndSlices(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	projectEntity, err := client.Project.Create().
		SetName("request-loader-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceEntity, err := client.Trace.Create().
		SetTraceID("trace-request-loader").
		SetProjectID(projectEntity.ID).
		Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		SetStatus("completed").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	responseBody, err := traceService.requestService.LoadResponseBody(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, responseBody)
	require.JSONEq(t, `{}`, string(responseBody))

	responseChunks, err := traceService.requestService.LoadResponseChunks(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, responseChunks)
	require.Empty(t, responseChunks)

	exec, err := client.RequestExecution.Create().
		SetProjectID(projectEntity.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"messages":[]}`)).
		SetStatus("completed").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	execResponseBody, err := traceService.requestService.LoadRequestExecutionResponseBody(ctx, exec)
	require.NoError(t, err)
	require.NotNil(t, execResponseBody)
	require.JSONEq(t, `{}`, string(execResponseBody))

	execResponseChunks, err := traceService.requestService.LoadRequestExecutionResponseChunks(ctx, exec)
	require.NoError(t, err)
	require.NotNil(t, execResponseChunks)
	require.Empty(t, execResponseChunks)
}

func TestTraceService_GetOrCreateTrace(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-test-123"

	// Test creating a new trace without thread
	trace1, err := traceService.GetOrCreateTrace(ctx, testProject.ID, traceID, nil)
	require.NoError(t, err)
	require.NotNil(t, trace1)
	require.Equal(t, traceID, trace1.TraceID)
	require.Equal(t, testProject.ID, trace1.ProjectID)

	// Test getting existing trace (should return the same trace)
	trace2, err := traceService.GetOrCreateTrace(ctx, testProject.ID, traceID, nil)
	require.NoError(t, err)
	require.NotNil(t, trace2)
	require.Equal(t, trace1.ID, trace2.ID)
	require.Equal(t, traceID, trace2.TraceID)
	require.Equal(t, testProject.ID, trace2.ProjectID)

	// Test creating a trace with different traceID
	differentTraceID := "trace-test-456"
	trace3, err := traceService.GetOrCreateTrace(ctx, testProject.ID, differentTraceID, nil)
	require.NoError(t, err)
	require.NotNil(t, trace3)
	require.NotEqual(t, trace1.ID, trace3.ID)
	require.Equal(t, differentTraceID, trace3.TraceID)
}

func TestTraceService_GetOrCreateTrace_WithThread(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create a thread
	testThread, err := client.Thread.Create().
		SetThreadID("thread-123").
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-with-thread-123"

	// Test creating a trace with thread
	trace, err := traceService.GetOrCreateTrace(ctx, testProject.ID, traceID, &testThread.ID)
	require.NoError(t, err)
	require.NotNil(t, trace)
	require.Equal(t, traceID, trace.TraceID)
	require.Equal(t, testProject.ID, trace.ProjectID)
	require.Equal(t, testThread.ID, trace.ThreadID)
}

func TestTraceService_GetOrCreateTrace_DifferentProjects(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create two test projects
	project1, err := client.Project.Create().
		SetName("project-1").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	project2, err := client.Project.Create().
		SetName("project-2").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Use different trace IDs for different projects (trace_id is globally unique)
	traceID1 := "trace-project1-123"
	traceID2 := "trace-project2-456"

	// Create trace in project 1
	trace1, err := traceService.GetOrCreateTrace(ctx, project1.ID, traceID1, nil)
	require.NoError(t, err)
	require.Equal(t, project1.ID, trace1.ProjectID)
	require.Equal(t, traceID1, trace1.TraceID)

	// Create trace in project 2 with different traceID
	trace2, err := traceService.GetOrCreateTrace(ctx, project2.ID, traceID2, nil)
	require.NoError(t, err)
	require.Equal(t, project2.ID, trace2.ProjectID)
	require.Equal(t, traceID2, trace2.TraceID)
	require.NotEqual(t, trace1.ID, trace2.ID)
}

func TestTraceService_GetTraceByID(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-get-test-123"

	// Create a trace first
	createdTrace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Test getting the trace
	retrievedTrace, err := traceService.GetTraceByID(ctx, traceID, testProject.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedTrace)
	require.Equal(t, createdTrace.ID, retrievedTrace.ID)
	require.Equal(t, traceID, retrievedTrace.TraceID)

	// Test getting non-existent trace
	_, err = traceService.GetTraceByID(ctx, "non-existent", testProject.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get trace")
}

func TestTraceService_GetRequestTrace(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-spans-test-123"

	// Create a trace
	trace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create test request with simple text message
	requestBody := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello, how are you?"}
		]
	}`)

	responseBody := []byte(`{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "I'm doing well, thank you!"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30
		}
	}`)

	req, err := client.Request.Create().
		SetProjectID(testProject.ID).
		SetTraceID(trace.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(requestBody).
		SetResponseBody(responseBody).
		SetStatus("completed").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Test GetRequestTrace
	traceRoot, err := traceService.GetRootSegment(ctx, trace.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Verify request trace structure
	require.Equal(t, req.ID, traceRoot.ID)
	require.Nil(t, traceRoot.ParentID)
	require.Len(t, traceRoot.Children, 0)
	require.NotZero(t, traceRoot.StartTime)
	require.NotZero(t, traceRoot.EndTime)

	// Verify spans
	require.NotEmpty(t, traceRoot.RequestSpans)
	require.NotNil(t, findSpanByType(traceRoot.RequestSpans, "user_query"))
	require.NotEmpty(t, traceRoot.ResponseSpans)
	require.NotNil(t, findSpanByType(traceRoot.ResponseSpans, "text"))

	// Metadata should be populated from the response usage
	require.NotNil(t, traceRoot.Metadata)
	require.NotNil(t, traceRoot.Metadata.InputTokens)
	require.Equal(t, int64(10), *traceRoot.Metadata.InputTokens)
	require.NotNil(t, traceRoot.Metadata.OutputTokens)
	require.Equal(t, int64(20), *traceRoot.Metadata.OutputTokens)
}

func TestTraceService_GetRequestTrace_WithToolCalls(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-tool-test-456"

	// Create a trace
	trace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create request with tool calls
	requestBody := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "What's the weather?"}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get weather information"
				}
			}
		]
	}`)

	responseBody := []byte(`{
		"id": "chatcmpl-456",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_123",
					"type": "function",
					"function": {
						"name": "get_weather",
						"arguments": "{\"location\": \"San Francisco\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {
			"prompt_tokens": 15,
			"completion_tokens": 25,
			"total_tokens": 40
		}
	}`)

	_, err = client.Request.Create().
		SetProjectID(testProject.ID).
		SetTraceID(trace.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(requestBody).
		SetResponseBody(responseBody).
		SetStatus("completed").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, trace.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Ensure request spans still capture the original user message
	require.NotNil(t, findSpanByType(traceRoot.RequestSpans, "user_query"))

	// Tool calls from the assistant should be captured in the response spans
	toolSpan := findSpanByType(traceRoot.ResponseSpans, "tool_use")
	require.NotNil(t, toolSpan, "expected tool_use span in response spans")
	require.NotNil(t, toolSpan.Value)
	require.NotNil(t, toolSpan.Value.ToolUse)
	require.Equal(t, "get_weather", toolSpan.Value.ToolUse.Name)
}

func TestTraceService_GetRequestTrace_AnthropicResponseTransformation(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	projectEntity, err := client.Project.Create().
		SetName("anthropic-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-anthropic-response"
	traceEntity, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(projectEntity.ID).
		Save(ctx)
	require.NoError(t, err)

	anthropicRequest := []byte(`{
		"model": "claude-3-sonnet-20240229",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "Summarize the following."}
				]
			}
		]
	}`)

	anthropicResponse := []byte(`{
		"id": "msg-123",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-sonnet-20240229",
		"content": [
			{"type": "thinking", "thinking": "Analyzing the request"},
			{"type": "text", "text": "Here is the summary."},
			{"type": "tool_use", "id": "tool_1", "name": "get_weather", "input": {"location": "San Francisco"}}
		],
		"usage": {
			"input_tokens": 12,
			"output_tokens": 18
		},
		"stop_reason": "tool_use"
	}`)

	_, err = client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("claude-3-sonnet-20240229").
		SetFormat("anthropic/messages").
		SetRequestBody(anthropicRequest).
		SetResponseBody(anthropicResponse).
		SetStatus("completed").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Metadata should be populated from the response usage
	require.NotNil(t, traceRoot.Metadata)
	require.NotNil(t, traceRoot.Metadata.InputTokens)
	require.Equal(t, int64(12), *traceRoot.Metadata.InputTokens)
	require.NotNil(t, traceRoot.Metadata.OutputTokens)
	require.Equal(t, int64(18), *traceRoot.Metadata.OutputTokens)

	// The original user query should be in the request spans
	require.NotNil(t, findSpanByType(traceRoot.RequestSpans, "user_query"))

	// Anthropic responses expose content blocks via response spans
	textSpan := findSpanByType(traceRoot.ResponseSpans, "text")
	require.NotNil(t, textSpan, "expected text span from anthropic response")
	require.NotNil(t, textSpan.Value)
	require.NotNil(t, textSpan.Value.Text)
	require.NotEmpty(t, textSpan.Value.Text.Text)

	toolSpan := findSpanByType(traceRoot.ResponseSpans, "tool_use")
	require.NotNil(t, toolSpan, "expected tool_use span from anthropic response")
	require.NotNil(t, toolSpan.Value)
	require.NotNil(t, toolSpan.Value.ToolUse)
	require.Equal(t, "get_weather", toolSpan.Value.ToolUse.Name)
}

func TestTraceService_GetRequestTrace_WithReasoningContent(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-reasoning-test-789"

	// Create a trace
	trace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create request with reasoning content
	requestBody := []byte(`{
		"model": "deepseek-reasoner",
		"messages": [
			{"role": "user", "content": "Solve this math problem"}
		]
	}`)

	responseBody := []byte(`{
		"id": "chatcmpl-789",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "deepseek-reasoner",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "The answer is 42",
				"reasoning_content": "Let me think through this step by step..."
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 12,
			"completion_tokens": 28,
			"total_tokens": 40
		}
	}`)

	_, err = client.Request.Create().
		SetProjectID(testProject.ID).
		SetTraceID(trace.ID).
		SetModelID("deepseek-reasoner").
		SetFormat("openai/chat_completions").
		SetRequestBody(requestBody).
		SetResponseBody(responseBody).
		SetStatus("completed").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, trace.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Reasoning content should be exposed as a thinking span in the response
	thinkingSpan := findSpanByType(traceRoot.ResponseSpans, "thinking")
	require.NotNil(t, thinkingSpan, "expected thinking span in response")
	require.NotNil(t, thinkingSpan.Value)
	require.NotNil(t, thinkingSpan.Value.Thinking)
	require.Contains(t, thinkingSpan.Value.Thinking.Thinking, "Let me think")
}

func TestDeduplicateSpansWithParent_CompactSummaryUsesContentKey(t *testing.T) {
	parent := []Span{{
		ID:   "parent-compact",
		Type: "compaction",
		Value: &SpanValue{
			Compaction: &SpanCompaction{Summary: "summary-a"},
		},
	}}

	current := []Span{{
		ID:   "child-compact",
		Type: "compaction",
		Value: &SpanValue{
			Compaction: &SpanCompaction{Summary: "summary-b"},
		},
	}}

	result := deduplicateSpansWithParent(current, parent)
	require.Len(t, result, 1)
	require.Equal(t, "summary-b", result[0].Value.Compaction.Summary)
}

func TestSpanToKey_CompactTypesIncludeSummary(t *testing.T) {
	tests := []struct {
		name string
		span Span
		want string
	}{
		{
			name: "compaction",
			span: Span{
				Type: "compaction",
				Value: &SpanValue{
					Compaction: &SpanCompaction{Summary: "compact-a"},
				},
			},
			want: "compaction:compact-a",
		},
		{
			name: "compaction_summary",
			span: Span{
				Type: "compaction_summary",
				Value: &SpanValue{
					Compaction: &SpanCompaction{Summary: "compact-b"},
				},
			},
			want: "compaction_summary:compact-b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, spanToKey(tt.span))
		})
	}
}

func TestTraceService_GetRequestTrace_EmptyTrace(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-empty-test"

	// Create a trace without any requests
	trace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, trace.ID)
	require.NoError(t, err)
	require.Nil(t, traceRoot)
}

func TestTraceService_GetRequestTrace_MultipleRequestsWithToolResults(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	projectEntity, err := client.Project.Create().
		SetName("multi-request-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceEntity, err := client.Trace.Create().
		SetTraceID("trace-multi-request").
		SetProjectID(projectEntity.ID).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()

	request1Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "What is the weather in New York and what is 15 * 23?"},
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{
						"id": "call_weather",
						"type": "function",
						"function": {
							"name": "get_current_weather",
							"arguments": "{\"location\": \"New York\"}"
						}
					},
					{
						"id": "call_calculate",
						"type": "function",
						"function": {
							"name": "calculate",
							"arguments": "{\"expression\": \"15 * 23\"}"
						}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_weather",
				"content": "Current weather in New York: 22°C, Partly cloudy, humidity 65%"
			},
			{
				"role": "tool",
				"tool_call_id": "call_calculate",
				"content": "345"
			}
		]
	}`)

	response1Body := []byte(`{
		"id": "chatcmpl-001",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "The current weather in New York is 22°C, partly cloudy, with a humidity of 65%. The result of 15 * 23 is 345."
			},
			"finish_reason": "stop"
		}]
	}`)

	request2Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Thanks! Can you also give me tomorrow's forecast?"},
			{"role": "assistant", "content": "Sure, here is the forecast for tomorrow."}
		]
	}`)

	response2Body := []byte(`{
		"id": "chatcmpl-002",
		"object": "chat.completion",
		"created": 1677652290,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Tomorrow will be mostly sunny with a high of 24°C."
			},
			"finish_reason": "stop"
		}]
	}`)

	req1, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(request1Body).
		SetResponseBody(response1Body).
		SetStatus("completed").
		SetStream(false).
		SetCreatedAt(now).
		SetUpdatedAt(now.Add(100 * time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	req2, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(request2Body).
		SetResponseBody(response2Body).
		SetStatus("completed").
		SetStream(false).
		SetCreatedAt(now.Add(time.Second)).
		SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Root request should be the first one chronologically
	require.Equal(t, req1.ID, traceRoot.ID)
	require.Len(t, traceRoot.Children, 1)

	child := traceRoot.Children[0]
	require.Equal(t, req2.ID, child.ID)
	require.NotNil(t, child.ParentID)
	require.Equal(t, req1.ID, *child.ParentID)

	// Ensure tool calls and tool results are captured on the first request
	require.Equal(t, 2, countSpansByType(traceRoot.RequestSpans, "tool_use"))
	require.Equal(t, 2, countSpansByType(traceRoot.RequestSpans, "tool_result"))

	// The first request should also have a final assistant response span
	require.NotNil(t, findSpanByType(traceRoot.ResponseSpans, "text"))

	// The follow-up request should capture the user query and assistant reply
	require.NotNil(t, findSpanByType(child.RequestSpans, "user_query"))
	require.NotNil(t, findSpanByType(child.ResponseSpans, "text"))
}

func TestTraceService_GetRootSegment_TreeByToolCallID(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	projectEntity, err := client.Project.Create().
		SetName("tree-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceEntity, err := client.Trace.Create().
		SetTraceID("trace-tree-toolcall").
		SetProjectID(projectEntity.ID).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()

	// Request 1: user asks → LLM responds with two tool_calls (call_A, call_B)
	req1Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Do tasks A and B"}
		]
	}`)
	resp1Body := []byte(`{
		"id": "chatcmpl-001",
		"object": "chat.completion",
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [
					{"id": "call_A", "type": "function", "function": {"name": "task_a", "arguments": "{}"}},
					{"id": "call_B", "type": "function", "function": {"name": "task_b", "arguments": "{}"}}
				]
			},
			"finish_reason": "tool_calls"
		}]
	}`)

	// Request 2: carries Request 1 context + tool_result for call_A only
	req2Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Do tasks A and B"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_A", "type": "function", "function": {"name": "task_a", "arguments": "{}"}},
				{"id": "call_B", "type": "function", "function": {"name": "task_b", "arguments": "{}"}}
			]},
			{"role": "tool", "tool_call_id": "call_A", "content": "Task A done"}
		]
	}`)
	resp2Body := []byte(`{
		"id": "chatcmpl-002",
		"object": "chat.completion",
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Task A is done."},
			"finish_reason": "stop"
		}]
	}`)

	// Request 3: carries Request 1 context + tool_result for call_B only
	req3Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Do tasks A and B"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_A", "type": "function", "function": {"name": "task_a", "arguments": "{}"}},
				{"id": "call_B", "type": "function", "function": {"name": "task_b", "arguments": "{}"}}
			]},
			{"role": "tool", "tool_call_id": "call_B", "content": "Task B done"}
		]
	}`)
	resp3Body := []byte(`{
		"id": "chatcmpl-003",
		"object": "chat.completion",
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Task B is done."},
			"finish_reason": "stop"
		}]
	}`)

	req1, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(req1Body).
		SetResponseBody(resp1Body).
		SetStatus("completed").
		SetStream(false).
		SetCreatedAt(now).
		SetUpdatedAt(now.Add(100 * time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	req2, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(req2Body).
		SetResponseBody(resp2Body).
		SetStatus("completed").
		SetStream(false).
		SetCreatedAt(now.Add(time.Second)).
		SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	req3, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody(req3Body).
		SetResponseBody(resp3Body).
		SetStatus("completed").
		SetStream(false).
		SetCreatedAt(now.Add(2 * time.Second)).
		SetUpdatedAt(now.Add(2*time.Second + 100*time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Root should be Request 1
	require.Equal(t, req1.ID, traceRoot.ID)
	require.Nil(t, traceRoot.ParentID)

	// Both Request 2 and Request 3 should be children of Request 1 (tree, not chain)
	// because both consume tool_call_ids produced by Request 1
	require.Len(t, traceRoot.Children, 2, "expected 2 children for root (tree structure), got a chain")

	child1 := traceRoot.Children[0]
	child2 := traceRoot.Children[1]

	require.Equal(t, req2.ID, child1.ID)
	require.Equal(t, req3.ID, child2.ID)
	require.Equal(t, req1.ID, *child1.ParentID)
	require.Equal(t, req1.ID, *child2.ParentID)

	// Children should have no further children
	require.Len(t, child1.Children, 0)
	require.Len(t, child2.Children, 0)

	// Verify deduplicated request spans: the shared prefix (user_query + tool_calls) should be removed
	// Only the unique tool_result span should remain
	require.Equal(t, 1, countSpansByType(child1.RequestSpans, "tool_result"))
	require.Equal(t, 1, countSpansByType(child2.RequestSpans, "tool_result"))
}

func TestTraceService_GetRootSegment_TreeBySpanPrefixMatch(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	projectEntity, err := client.Project.Create().
		SetName("prefix-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceEntity, err := client.Trace.Create().
		SetTraceID("trace-prefix-match").
		SetProjectID(projectEntity.ID).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()

	// Request 1: [user_msg] → text response
	req1Body := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "Hello"}]
	}`)
	resp1Body := []byte(`{
		"id": "c-001", "object": "chat.completion", "model": "gpt-4",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi there!"}, "finish_reason": "stop"}]
	}`)

	// Request 2: carries Request 1 full context + new user message (prefix matches Request 1)
	req2Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
			{"role": "user", "content": "Tell me more"}
		]
	}`)
	resp2Body := []byte(`{
		"id": "c-002", "object": "chat.completion", "model": "gpt-4",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Sure, here is more info."}, "finish_reason": "stop"}]
	}`)

	// Request 3: carries Request 1+2 full context + new user message (prefix matches Request 2)
	req3Body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
			{"role": "user", "content": "Tell me more"},
			{"role": "assistant", "content": "Sure, here is more info."},
			{"role": "user", "content": "Thanks!"}
		]
	}`)
	resp3Body := []byte(`{
		"id": "c-003", "object": "chat.completion", "model": "gpt-4",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "You are welcome!"}, "finish_reason": "stop"}]
	}`)

	req1, err := client.Request.Create().
		SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").SetFormat("openai/chat_completions").
		SetRequestBody(req1Body).SetResponseBody(resp1Body).
		SetStatus("completed").SetStream(false).
		SetCreatedAt(now).SetUpdatedAt(now.Add(100 * time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	req2, err := client.Request.Create().
		SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").SetFormat("openai/chat_completions").
		SetRequestBody(req2Body).SetResponseBody(resp2Body).
		SetStatus("completed").SetStream(false).
		SetCreatedAt(now.Add(time.Second)).SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	req3, err := client.Request.Create().
		SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
		SetModelID("gpt-4").SetFormat("openai/chat_completions").
		SetRequestBody(req3Body).SetResponseBody(resp3Body).
		SetStatus("completed").SetStream(false).
		SetCreatedAt(now.Add(2 * time.Second)).SetUpdatedAt(now.Add(2*time.Second + 100*time.Millisecond)).
		Save(ctx)
	require.NoError(t, err)

	traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
	require.NoError(t, err)
	require.NotNil(t, traceRoot)

	// Request 1 is root
	require.Equal(t, req1.ID, traceRoot.ID)

	// Request 2's prefix matches Request 1 (longest match) → child of Request 1
	require.Len(t, traceRoot.Children, 1)
	child := traceRoot.Children[0]
	require.Equal(t, req2.ID, child.ID)
	require.Equal(t, req1.ID, *child.ParentID)

	// Request 3's prefix matches Request 2 more than Request 1 → child of Request 2
	require.Len(t, child.Children, 1)
	grandchild := child.Children[0]
	require.Equal(t, req3.ID, grandchild.ID)
	require.Equal(t, req2.ID, *grandchild.ParentID)

	// Verify deduplication: each child should only have the new unique spans
	// Request 2 should have 1 new user_query ("Tell me more"), not the shared prefix
	require.Equal(t, 1, countSpansByType(child.RequestSpans, "user_query"))
	// Request 3 should have 1 new user_query ("Thanks!"), not the shared prefix
	require.Equal(t, 1, countSpansByType(grandchild.RequestSpans, "user_query"))
}

func TestTraceService_GetRootSegment_FallbackChronologicalNearest(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	projectEntity, err := client.Project.Create().
		SetName("fallback-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()

	t.Run("two disjoint requests form a linear chain", func(t *testing.T) {
		traceEntity, err := client.Trace.Create().
			SetTraceID("trace-fallback-2").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		// Request 1: standalone user message
		req1Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Alpha question"}]
		}`)
		resp1Body := []byte(`{
			"id": "c-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Alpha answer"}, "finish_reason": "stop"}]
		}`)

		// Request 2: completely different user message — no shared prefix, no tool_call_ids
		req2Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Beta question"}]
		}`)
		resp2Body := []byte(`{
			"id": "c-002", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Beta answer"}, "finish_reason": "stop"}]
		}`)

		req1, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req1Body).SetResponseBody(resp1Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now).SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		req2, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req2Body).SetResponseBody(resp2Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(time.Second)).SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
		require.NoError(t, err)
		require.NotNil(t, traceRoot)

		// Request 1 is root
		require.Equal(t, req1.ID, traceRoot.ID)
		require.Nil(t, traceRoot.ParentID)

		// Request 2 falls back to the chronologically nearest previous (Request 1)
		require.Len(t, traceRoot.Children, 1)
		child := traceRoot.Children[0]
		require.Equal(t, req2.ID, child.ID)
		require.Equal(t, req1.ID, *child.ParentID)
		require.Len(t, child.Children, 0)
	})

	t.Run("three disjoint requests form a linear chain via fallback", func(t *testing.T) {
		traceEntity, err := client.Trace.Create().
			SetTraceID("trace-fallback-3").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		// Three requests with completely disjoint content — no shared prefix, no tool_call_ids
		req1Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "First topic"}]
		}`)
		resp1Body := []byte(`{
			"id": "c-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "First reply"}, "finish_reason": "stop"}]
		}`)

		req2Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Second topic"}]
		}`)
		resp2Body := []byte(`{
			"id": "c-002", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Second reply"}, "finish_reason": "stop"}]
		}`)

		req3Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Third topic"}]
		}`)
		resp3Body := []byte(`{
			"id": "c-003", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Third reply"}, "finish_reason": "stop"}]
		}`)

		req1, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req1Body).SetResponseBody(resp1Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now).SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		req2, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req2Body).SetResponseBody(resp2Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(time.Second)).SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		req3, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req3Body).SetResponseBody(resp3Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(2 * time.Second)).SetUpdatedAt(now.Add(2*time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
		require.NoError(t, err)
		require.NotNil(t, traceRoot)

		// Request 1 is root
		require.Equal(t, req1.ID, traceRoot.ID)
		require.Nil(t, traceRoot.ParentID)

		// Request 2 falls back to nearest predecessor → Request 1
		require.Len(t, traceRoot.Children, 1)
		child := traceRoot.Children[0]
		require.Equal(t, req2.ID, child.ID)
		require.Equal(t, req1.ID, *child.ParentID)

		// Request 3 falls back to nearest predecessor → Request 2 (not Request 1)
		require.Len(t, child.Children, 1)
		grandchild := child.Children[0]
		require.Equal(t, req3.ID, grandchild.ID)
		require.Equal(t, req2.ID, *grandchild.ParentID)
		require.Len(t, grandchild.Children, 0)
	})

	t.Run("fallback is superseded when prefix match exists", func(t *testing.T) {
		traceEntity, err := client.Trace.Create().
			SetTraceID("trace-fallback-mixed").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		// Request 1: [user_msg_A]
		req1Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Alpha"}]
		}`)
		resp1Body := []byte(`{
			"id": "c-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Alpha reply"}, "finish_reason": "stop"}]
		}`)

		// Request 2: completely different content — no prefix match with Request 1
		req2Body := []byte(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Bravo"}]
		}`)
		resp2Body := []byte(`{
			"id": "c-002", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Bravo reply"}, "finish_reason": "stop"}]
		}`)

		// Request 3: prefix matches Request 1 (carries Request 1 context)
		req3Body := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "user", "content": "Alpha"},
				{"role": "assistant", "content": "Alpha reply"},
				{"role": "user", "content": "Continue Alpha"}
			]
		}`)
		resp3Body := []byte(`{
			"id": "c-003", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Alpha continued"}, "finish_reason": "stop"}]
		}`)

		req1, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req1Body).SetResponseBody(resp1Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now).SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		req2, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req2Body).SetResponseBody(resp2Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(time.Second)).SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		req3, err := client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(req3Body).SetResponseBody(resp3Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(2 * time.Second)).SetUpdatedAt(now.Add(2*time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		traceRoot, err := traceService.GetRootSegment(ctx, traceEntity.ID)
		require.NoError(t, err)
		require.NotNil(t, traceRoot)

		// Request 1 is root
		require.Equal(t, req1.ID, traceRoot.ID)

		// Request 2: no prefix match → fallback to nearest predecessor (Request 1)
		// Request 3: prefix matches Request 1 → child of Request 1 (not Request 2)
		// So both Request 2 and Request 3 should be children of Request 1
		require.Len(t, traceRoot.Children, 2)

		child1 := traceRoot.Children[0]
		child2 := traceRoot.Children[1]

		require.Equal(t, req2.ID, child1.ID)
		require.Equal(t, req1.ID, *child1.ParentID)
		require.Equal(t, req3.ID, child2.ID)
		require.Equal(t, req1.ID, *child2.ParentID)

		require.Len(t, child1.Children, 0)
		require.Len(t, child2.Children, 0)
	})
}

func TestTraceService_GetRootSegment_CrossTraceDedup(t *testing.T) {
	t.Run("first segment deduplicates against previous trace in same thread", func(t *testing.T) {
		traceService, client := setupTestTraceService(t, nil)
		defer client.Close()

		ctx := context.Background()
		ctx = ent.NewContext(ctx, client)
		ctx = authz.WithTestBypass(ctx)

		projectEntity, err := client.Project.Create().
			SetName("cross-trace-project").
			SetStatus(project.StatusActive).
			Save(ctx)
		require.NoError(t, err)

		threadEntity, err := client.Thread.Create().
			SetThreadID("thread-cross-trace-dedup").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		now := time.Now()

		// --- Trace 1: user asks a question, LLM responds ---
		trace1, err := client.Trace.Create().
			SetTraceID("trace-cross-1").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now).
			Save(ctx)
		require.NoError(t, err)

		trace1ReqBody := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "What is Go?"}
			]
		}`)
		trace1RespBody := []byte(`{
			"id": "c-t1-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Go is a programming language."}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).
			SetTraceID(trace1.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody(trace1ReqBody).
			SetResponseBody(trace1RespBody).
			SetStatus("completed").
			SetStream(false).
			SetCreatedAt(now).
			SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		// --- Trace 2: carries trace 1's full context + new user message ---
		trace2, err := client.Trace.Create().
			SetTraceID("trace-cross-2").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now.Add(10 * time.Second)).
			Save(ctx)
		require.NoError(t, err)

		trace2ReqBody := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "What is Go?"},
				{"role": "assistant", "content": "Go is a programming language."},
				{"role": "user", "content": "Tell me about goroutines."}
			]
		}`)
		trace2RespBody := []byte(`{
			"id": "c-t2-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Goroutines are lightweight threads."}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 40, "completion_tokens": 10, "total_tokens": 50}
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).
			SetTraceID(trace2.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody(trace2ReqBody).
			SetResponseBody(trace2RespBody).
			SetStatus("completed").
			SetStream(false).
			SetCreatedAt(now.Add(10 * time.Second)).
			SetUpdatedAt(now.Add(10*time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		// Verify trace 2's root segment has duplicated spans removed
		root, err := traceService.GetRootSegment(ctx, trace2.ID)
		require.NoError(t, err)
		require.NotNil(t, root)

		// Only the new user_query ("Tell me about goroutines.") should remain
		// The system instruction, first user_query, and assistant text from trace 1 should be removed
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "user_query"),
			"expected only the new user_query, previous trace context should be deduped")
		require.Equal(t, 0, countSpansByType(root.RequestSpans, "system_instruction"),
			"system instruction from previous trace should be deduped")
		require.Equal(t, 0, countSpansByType(root.RequestSpans, "text"),
			"assistant text from previous trace should be deduped")

		// The new user query should be the correct one
		userSpan := findSpanByType(root.RequestSpans, "user_query")
		require.NotNil(t, userSpan)
		require.Equal(t, "Tell me about goroutines.", userSpan.Value.UserQuery.Text)

		// Response spans should be unaffected
		require.Equal(t, 1, countSpansByType(root.ResponseSpans, "text"))
	})

	t.Run("no dedup when trace has no thread", func(t *testing.T) {
		traceService, client := setupTestTraceService(t, nil)
		defer client.Close()

		ctx := context.Background()
		ctx = ent.NewContext(ctx, client)
		ctx = authz.WithTestBypass(ctx)

		projectEntity, err := client.Project.Create().
			SetName("no-thread-project").
			SetStatus(project.StatusActive).
			Save(ctx)
		require.NoError(t, err)

		now := time.Now()

		traceEntity, err := client.Trace.Create().
			SetTraceID("trace-no-thread").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		reqBody := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "system", "content": "You are helpful."},
				{"role": "user", "content": "Hello"}
			]
		}`)
		respBody := []byte(`{
			"id": "c-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi!"}, "finish_reason": "stop"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).
			SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody(reqBody).
			SetResponseBody(respBody).
			SetStatus("completed").
			SetStream(false).
			SetCreatedAt(now).
			SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		root, err := traceService.GetRootSegment(ctx, traceEntity.ID)
		require.NoError(t, err)
		require.NotNil(t, root)

		// All spans should remain since there's no thread context to dedup against
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "system_instruction"))
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "user_query"))
	})

	t.Run("no dedup when trace is the first in thread", func(t *testing.T) {
		traceService, client := setupTestTraceService(t, nil)
		defer client.Close()

		ctx := context.Background()
		ctx = ent.NewContext(ctx, client)
		ctx = authz.WithTestBypass(ctx)

		projectEntity, err := client.Project.Create().
			SetName("first-in-thread-project").
			SetStatus(project.StatusActive).
			Save(ctx)
		require.NoError(t, err)

		threadEntity, err := client.Thread.Create().
			SetThreadID("thread-first-trace").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		now := time.Now()

		traceEntity, err := client.Trace.Create().
			SetTraceID("trace-first-in-thread").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now).
			Save(ctx)
		require.NoError(t, err)

		reqBody := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "system", "content": "You are helpful."},
				{"role": "user", "content": "Hello"}
			]
		}`)
		respBody := []byte(`{
			"id": "c-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi!"}, "finish_reason": "stop"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).
			SetTraceID(traceEntity.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody(reqBody).
			SetResponseBody(respBody).
			SetStatus("completed").
			SetStream(false).
			SetCreatedAt(now).
			SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		root, err := traceService.GetRootSegment(ctx, traceEntity.ID)
		require.NoError(t, err)
		require.NotNil(t, root)

		// All spans should remain since this is the first trace in the thread
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "system_instruction"))
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "user_query"))
	})

	t.Run("cross-trace dedup with tool calls in previous trace", func(t *testing.T) {
		traceService, client := setupTestTraceService(t, nil)
		defer client.Close()

		ctx := context.Background()
		ctx = ent.NewContext(ctx, client)
		ctx = authz.WithTestBypass(ctx)

		projectEntity, err := client.Project.Create().
			SetName("cross-trace-tools-project").
			SetStatus(project.StatusActive).
			Save(ctx)
		require.NoError(t, err)

		threadEntity, err := client.Thread.Create().
			SetThreadID("thread-cross-trace-tools").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		now := time.Now()

		// --- Trace 1: Request 1 (user ask) → tool call ---
		trace1, err := client.Trace.Create().
			SetTraceID("trace-tools-1").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now).
			Save(ctx)
		require.NoError(t, err)

		t1Req1Body := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "user", "content": "What is the weather?"}
			]
		}`)
		t1Resp1Body := []byte(`{
			"id": "c-t1-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": null,
				"tool_calls": [{"id": "call_weather", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}}]
			}, "finish_reason": "tool_calls"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(trace1.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(t1Req1Body).SetResponseBody(t1Resp1Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now).SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		// Trace 1: Request 2 (tool result → final answer)
		t1Req2Body := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "user", "content": "What is the weather?"},
				{"role": "assistant", "content": null, "tool_calls": [{"id": "call_weather", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}}]},
				{"role": "tool", "tool_call_id": "call_weather", "content": "72°F sunny"}
			]
		}`)
		t1Resp2Body := []byte(`{
			"id": "c-t1-002", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "The weather in SF is 72°F and sunny."}, "finish_reason": "stop"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(trace1.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(t1Req2Body).SetResponseBody(t1Resp2Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(time.Second)).SetUpdatedAt(now.Add(time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		// --- Trace 2: carries all of trace 1's context + new user message ---
		trace2, err := client.Trace.Create().
			SetTraceID("trace-tools-2").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now.Add(10 * time.Second)).
			Save(ctx)
		require.NoError(t, err)

		t2ReqBody := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "user", "content": "What is the weather?"},
				{"role": "assistant", "content": null, "tool_calls": [{"id": "call_weather", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}}]},
				{"role": "tool", "tool_call_id": "call_weather", "content": "72°F sunny"},
				{"role": "assistant", "content": "The weather in SF is 72°F and sunny."},
				{"role": "user", "content": "What about tomorrow?"}
			]
		}`)
		t2RespBody := []byte(`{
			"id": "c-t2-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Tomorrow will be 68°F and cloudy."}, "finish_reason": "stop"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(trace2.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(t2ReqBody).SetResponseBody(t2RespBody).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(10 * time.Second)).SetUpdatedAt(now.Add(10*time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		root, err := traceService.GetRootSegment(ctx, trace2.ID)
		require.NoError(t, err)
		require.NotNil(t, root)

		// Only the new user_query should remain after dedup
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "user_query"),
			"only the new user query should remain")
		require.Equal(t, 0, countSpansByType(root.RequestSpans, "tool_use"),
			"tool_use from previous trace should be deduped")
		require.Equal(t, 0, countSpansByType(root.RequestSpans, "tool_result"),
			"tool_result from previous trace should be deduped")
		require.Equal(t, 0, countSpansByType(root.RequestSpans, "text"),
			"assistant text from previous trace should be deduped")

		userSpan := findSpanByType(root.RequestSpans, "user_query")
		require.NotNil(t, userSpan)
		require.Equal(t, "What about tomorrow?", userSpan.Value.UserQuery.Text)
	})

	t.Run("cross-trace dedup does not affect within-trace dedup", func(t *testing.T) {
		traceService, client := setupTestTraceService(t, nil)
		defer client.Close()

		ctx := context.Background()
		ctx = ent.NewContext(ctx, client)
		ctx = authz.WithTestBypass(ctx)

		projectEntity, err := client.Project.Create().
			SetName("cross-trace-combined-project").
			SetStatus(project.StatusActive).
			Save(ctx)
		require.NoError(t, err)

		threadEntity, err := client.Thread.Create().
			SetThreadID("thread-cross-trace-combined").
			SetProjectID(projectEntity.ID).
			Save(ctx)
		require.NoError(t, err)

		now := time.Now()

		// --- Trace 1: simple Q&A ---
		trace1, err := client.Trace.Create().
			SetTraceID("trace-combined-1").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(trace1.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4","messages":[{"role":"user","content":"First question"}]}`)).
			SetResponseBody([]byte(`{"id":"c1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"First answer"},"finish_reason":"stop"}]}`)).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now).SetUpdatedAt(now.Add(100 * time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		// --- Trace 2: two requests, first carries trace 1 context, second carries trace 2 req 1 context ---
		trace2, err := client.Trace.Create().
			SetTraceID("trace-combined-2").
			SetProjectID(projectEntity.ID).
			SetThreadID(threadEntity.ID).
			SetCreatedAt(now.Add(10 * time.Second)).
			Save(ctx)
		require.NoError(t, err)

		// Trace 2, Request 1: carries trace 1 context + new question
		t2Req1Body := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "user", "content": "First question"},
				{"role": "assistant", "content": "First answer"},
				{"role": "user", "content": "Second question"}
			]
		}`)
		t2Resp1Body := []byte(`{
			"id": "c-t2-001", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": null,
				"tool_calls": [{"id": "call_t2", "type": "function", "function": {"name": "search", "arguments": "{}"}}]
			}, "finish_reason": "tool_calls"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(trace2.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(t2Req1Body).SetResponseBody(t2Resp1Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(10 * time.Second)).SetUpdatedAt(now.Add(10*time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		// Trace 2, Request 2: carries trace 2 req 1 context + tool result
		t2Req2Body := []byte(`{
			"model": "gpt-4",
			"messages": [
				{"role": "user", "content": "First question"},
				{"role": "assistant", "content": "First answer"},
				{"role": "user", "content": "Second question"},
				{"role": "assistant", "content": null, "tool_calls": [{"id": "call_t2", "type": "function", "function": {"name": "search", "arguments": "{}"}}]},
				{"role": "tool", "tool_call_id": "call_t2", "content": "search result"}
			]
		}`)
		t2Resp2Body := []byte(`{
			"id": "c-t2-002", "object": "chat.completion", "model": "gpt-4",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Second answer"}, "finish_reason": "stop"}]
		}`)

		_, err = client.Request.Create().
			SetProjectID(projectEntity.ID).SetTraceID(trace2.ID).
			SetModelID("gpt-4").SetFormat("openai/chat_completions").
			SetRequestBody(t2Req2Body).SetResponseBody(t2Resp2Body).
			SetStatus("completed").SetStream(false).
			SetCreatedAt(now.Add(11 * time.Second)).SetUpdatedAt(now.Add(11*time.Second + 100*time.Millisecond)).
			Save(ctx)
		require.NoError(t, err)

		root, err := traceService.GetRootSegment(ctx, trace2.ID)
		require.NoError(t, err)
		require.NotNil(t, root)

		// Root (trace 2, req 1): cross-trace dedup removes trace 1 context,
		// only "Second question" should remain
		require.Equal(t, 1, countSpansByType(root.RequestSpans, "user_query"))
		userSpan := findSpanByType(root.RequestSpans, "user_query")
		require.NotNil(t, userSpan)
		require.Equal(t, "Second question", userSpan.Value.UserQuery.Text)

		// Child (trace 2, req 2): within-trace dedup against parent
		require.Len(t, root.Children, 1)
		child := root.Children[0]

		// Only the tool_result should remain after within-trace dedup
		require.Equal(t, 1, countSpansByType(child.RequestSpans, "tool_result"))
		require.Equal(t, 0, countSpansByType(child.RequestSpans, "user_query"),
			"user queries should be deduped against parent within-trace")
	})
}

func TestTraceService_GetRequestTrace_integration(t *testing.T) {
	if true {
		t.Skip("skipping integration test in short mode")
	}

	client := enttest.NewEntClient(t, "sqlite3", filepath.Join(xfile.ProjectDir(), "axonhub.db"))

	traceService, client := setupTestTraceService(t, client)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Test GetRequestTrace
	traceRoot, err := traceService.GetRootSegment(ctx, 153)
	require.NoError(t, err)

	data, err := json.Marshal(traceRoot)
	require.NoError(t, err)
	println(string(data))
}
