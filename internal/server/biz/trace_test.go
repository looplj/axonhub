package biz

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xfile"
)

func setupTestTraceService(t *testing.T, client *ent.Client) (*TraceService, *ent.Client) {
	t.Helper()

	if client == nil {
		client = enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
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
		RequestService: NewRequestService(client, systemService, usageLogService, dataStorageService),
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

func TestTraceService_GetOrCreateTrace(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

func TestTraceService_GetRequestTrace_EmptyTrace(t *testing.T) {
	traceService, client := setupTestTraceService(t, nil)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

func TestTraceService_GetRequestTrace_integration(t *testing.T) {
	if true {
		t.Skip("skipping integration test in short mode")
	}

	client := enttest.NewEntClient(t, "sqlite3", filepath.Join(xfile.ProjectDir(), "axonhub.db"))

	traceService, client := setupTestTraceService(t, client)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Test GetRequestTrace
	traceRoot, err := traceService.GetRootSegment(ctx, 153)
	require.NoError(t, err)

	data, err := json.Marshal(traceRoot)
	require.NoError(t, err)
	println(string(data))
}
