package gql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

// An aggregate over an empty request_executions table still returns one row,
// so the counts must come back as zero rather than an error.
func TestQueryExecutionSuccessCounts_EmptyTable(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	success, failed, err := resolver.queryExecutionSuccessCounts(ctx, nil, nil, false, time.UTC)
	require.NoError(t, err)
	require.Zero(t, success)
	require.Zero(t, failed)
}

// A transport failure never writes a usage log, so scoping executions through usage_logs
// would drop exactly those failures from the denominator and overstate the success rate.
// The API-key filter has to reach them through the request they belong to.
func TestQueryExecutionSuccessCounts_CountsFailuresWithoutAUsageLog(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	p, err := client.Project.Create().SetName("p").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusFailed).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	// No usage log is written for this one: the request never produced a billable result.
	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(req.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusFailed).
		SaveX(ctx)

	// A non-nil filter is required: buildAnalyticsExecutionWhere returns early on nil, so
	// a nil filter would skip the API-key subquery this test exists to exercise.
	success, failed, err := resolver.queryExecutionSuccessCounts(ctx, &AnalyticsFilter{}, []int{1}, false, time.UTC)
	require.NoError(t, err)
	require.Zero(t, success)
	require.Equal(t, 1, failed, "a failure with no usage log still belongs to the API key's window")
}
