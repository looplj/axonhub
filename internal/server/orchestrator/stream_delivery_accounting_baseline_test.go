package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestStreamDeliveryAccountingBaseline(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		eventType  string
		status     string
		usage      string
		wantStatus requestexecution.Status
		wantRows   int
	}{
		{"failed usage", "response.failed", "failed", `,"usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}`, requestexecution.StatusFailed, 1},
		{"incomplete zero", "response.incomplete", "incomplete", `,"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}`, requestexecution.StatusFailed, 1},
		{"canceled usage", "response.cancelled", "canceled", `,"usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}`, requestexecution.StatusCanceled, 1},
		{"failed missing usage", "response.failed", "failed", "", requestexecution.StatusFailed, 0},
		{"partial cancel usage", "response.in_progress", "in_progress", `,"usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}`, requestexecution.StatusCanceled, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			project := createTestProject(t, ctx, client)
			channel := createTestChannel(t, ctx, client)
			_, requestService, _, usageService := setupTestServices(t, client)
			req, err := client.Request.Create().SetProjectID(project.ID).SetChannelID(channel.ID).
				SetModelID("gpt-5").SetStatus(request.StatusProcessing).SetStream(true).
				SetRequestBody([]byte(`{"stream":true}`)).Save(ctx)
			require.NoError(t, err)
			exec, err := client.RequestExecution.Create().SetRequestID(req.ID).SetProjectID(project.ID).
				SetChannelID(channel.ID).SetModelID("gpt-5").SetFormat(llm.APIFormatOpenAIResponse.String()).
				SetStatus(requestexecution.StatusProcessing).SetStream(true).
				SetRequestBody([]byte(`{"stream":true}`)).Save(ctx)
			require.NoError(t, err)
			outbound, err := responses.NewOutboundTransformer("https://example.test", "synthetic-key")
			require.NoError(t, err)
			streamCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			event := &httpclient.StreamEvent{Type: scenario.eventType, Data: []byte(`{"type":"` + scenario.eventType + `","response":{"id":"resp_baseline","status":"` + scenario.status + `","output":[]` + scenario.usage + `}}`)}
			stream := NewOutboundPersistentStream(streamCtx, streams.SliceStream([]*httpclient.StreamEvent{event}), req, exec, requestService, usageService, outbound, nil, &PersistenceState{})
			// When
			require.True(t, stream.Next())
			stream.Current()
			cancel()
			require.NoError(t, stream.Close())
			// Then
			saved, err := client.RequestExecution.Get(ctx, exec.ID)
			require.NoError(t, err)
			require.Equal(t, scenario.wantStatus, saved.Status)
			rows, err := client.UsageLog.Query().All(ctx)
			require.NoError(t, err)
			require.Len(t, rows, scenario.wantRows)
		})
	}
}
