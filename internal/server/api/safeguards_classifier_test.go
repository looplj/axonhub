package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/internal/server/safeguards"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestSafeguardsClassifierPipelineRecordsUsageAndEnforcesAccess(t *testing.T) {
	f := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion,
		`{"id":"review","object":"chat.completion","model":"gpt-test","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"verdict"}}],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`, false)
	parent := f.handler(t).ChatCompletionOrchestrator
	parent.Inbound = anthropic.NewInboundTransformer()
	classify := newSafeguardsClassifier(parent)
	ctx := contexts.WithRequestID(f.ctx, "outer-request")
	projectID, _ := contexts.GetProjectID(ctx)
	ctx = contexts.WithAPIKey(ctx, &ent.APIKey{ID: 41, UserID: 31, ProjectID: projectID})
	text, err := classify(ctx, safeguards.ClassifierRequest{Model: "gpt-test", System: "policy", User: "evidence", MaxTokens: 4096})
	require.NoError(t, err)
	require.Equal(t, "verdict", text)
	f.assertDatabase(t, request.StatusCompleted, true)
	row, err := f.client.Request.Query().Only(f.ctx)
	require.NoError(t, err)
	require.Equal(t, 41, row.APIKeyID)
	require.Equal(t, "gpt-test", row.ModelID)
	require.NotEqual(t, "outer-request", row.ExternalID)
	usage, err := f.client.UsageLog.Query().Only(f.ctx)
	require.NoError(t, err)
	require.Equal(t, 41, usage.APIKeyID)
	_, ok := parent.Inbound.(*anthropic.InboundTransformer)
	require.True(t, ok, "classifier must not mutate the parent transformer")

	denied := &ent.APIKey{ID: 42, Profiles: &objects.APIKeyProfiles{ActiveProfile: "restricted", Profiles: []objects.APIKeyProfile{{Name: "restricted", ModelIDs: []string{"other-model"}}}}}
	ctx = contexts.WithAPIKey(ctx, denied)
	_, err = classify(ctx, safeguards.ClassifierRequest{Model: "gpt-test", System: "policy", User: "evidence", MaxTokens: 4096})
	require.Error(t, err)
	count, err := f.client.UsageLog.Query().Count(f.ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count, "denied reviewer must never consume upstream tokens")
}

func TestSafeguardsClassifierRequestIdentity(t *testing.T) {
	ctx := contexts.WithAPIKey(t.Context(), &ent.APIKey{ID: 42})
	ctx = contexts.WithProjectID(ctx, 7)
	ctx = contexts.WithRequestID(ctx, "parent-request")
	ctx = contexts.WithTraceID(ctx, "parent-trace")
	observed := false
	ctx = contexts.WithRequestRecordObserver(ctx, func(int) { observed = true })
	classify := safeguardsClassifierWithProcess(func(child context.Context, req *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		key, _ := contexts.GetAPIKey(child)
		require.Equal(t, 42, key.ID)
		project, _ := contexts.GetProjectID(child)
		require.Equal(t, 7, project)
		id, _ := contexts.GetRequestID(child)
		require.NotEqual(t, "parent-request", id)
		require.Equal(t, id, req.RequestID)
		trace, _ := contexts.GetTraceID(child)
		require.Equal(t, "parent-trace", trace)
		contexts.NotifyRequestRecord(child, 5)
		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, "reviewer", body["model"])
		require.Equal(t, false, body["stream"])
		require.Equal(t, float64(4096), body["max_tokens"])
		require.NotContains(t, body, "safeguards")
		require.Empty(t, req.Headers.Get("Authorization"))
		return orchestrator.ChatCompletionResult{ChatCompletion: &httpclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"choices":[{"finish_reason":"stop","message":{"content":"verdict"}}]}`)}}, nil
	})
	result, err := classify(ctx, safeguards.ClassifierRequest{Model: "reviewer", System: "policy", User: "evidence", MaxTokens: 4096})
	require.NoError(t, err)
	require.Equal(t, "verdict", result)
	require.False(t, observed)
	id, _ := contexts.GetRequestID(ctx)
	require.Equal(t, "parent-request", id)
}

func TestSafeguardsClassifierRejectsIncompleteOutput(t *testing.T) {
	for _, body := range []string{
		`{"choices":[]}`,
		`{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"content":"verdict","refusal":"refused"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"content":" "}}]}`,
		`not JSON`,
	} {
		t.Run(body, func(t *testing.T) {
			classify := safeguardsClassifierWithProcess(func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
				return orchestrator.ChatCompletionResult{ChatCompletion: &httpclient.Response{StatusCode: 200, Body: []byte(body)}}, nil
			})
			_, err := classify(t.Context(), safeguards.ClassifierRequest{})
			require.Error(t, err)
		})
	}
}

func TestSafeguardsClassifierPropagatesDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	classify := safeguardsClassifierWithProcess(func(child context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		return orchestrator.ChatCompletionResult{}, child.Err()
	})
	_, err := classify(ctx, safeguards.ClassifierRequest{})
	require.True(t, errors.Is(err, context.Canceled))
}

func TestSafeguardsFallbackUsesClassifierPipeline(t *testing.T) {
	f := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion,
		`{"id":"review","object":"chat.completion","model":"gpt-test","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"verdicts\":{\"tool1\":{\"flagged\":true,\"explanation\":\"Unauthorized deletion\"}}}"}}],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`, false)
	classify := newSafeguardsClassifier(f.handler(t).ChatCompletionOrchestrator)
	evaluator, err := safeguards.New([]byte(`{"safeguards":[{"type":"dangerous_tool_use"}]}`), safeguards.Config{
		ClassifierModel: "unavailable-reviewer", FallbackModels: []string{"gpt-test"},
	}, classify)
	require.NoError(t, err)
	results := evaluator.Evaluate(f.ctx, []safeguards.ToolUse{{ID: "tool1", Name: "Bash", Input: json.RawMessage(`{"command":"rm -rf /tmp/data"}`)}})
	require.Equal(t, "flagged", results[0].Status.ToolUses["tool1"].Outcome)
	usage, err := f.client.UsageLog.Query().Only(f.ctx)
	require.NoError(t, err)
	require.Equal(t, "gpt-test", usage.ModelID)
	require.Equal(t, int64(46), usage.TotalTokens)
}
