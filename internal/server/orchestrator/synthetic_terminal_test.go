package orchestrator

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// syntheticFixturePath locates the captured synthetic.new upstream body (final
// "data: [DONE]" line with no trailing newline).
func syntheticFixturePath() string {
	return filepath.Join("..", "..", "..", "llm", "httpclient", "testdata", "synthetic_upstream_unterminated_final_line.raw")
}

// driveSyntheticChain runs the captured synthetic.new bytes through the real
// decoder and the real openai outbound + inbound transformers, with the
// production persistence policy (the ent request body is empty, as with
// store_request_body:false). It returns the client-facing events and the
// persisted request.
func driveSyntheticChain(t *testing.T, client *ent.Client, ctx context.Context,
	projectID, channelID int, state *PersistenceState,
) ([]*httpclient.StreamEvent, *ent.Request) {
	t.Helper()

	body, err := os.ReadFile(syntheticFixturePath())
	require.NoError(t, err)

	raw := httpclient.NewDefaultSSEDecoder(ctx, io.NopCloser(bytes.NewReader(body)))

	outbound, err := openai.NewOutboundTransformer("https://provider.test", "key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(ctx, &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIChatCompletion)}, raw)
	require.NoError(t, err)

	inbound := &PersistentInboundTransformer{wrapped: openai.NewInboundTransformer(), state: state}
	// The live in-flight request carries the real (valid) body; only the
	// persisted ent row is empty, matching store_request_body:false.
	_, err = inbound.TransformRequest(ctx, &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		Headers:   http.Header{"Content-Type": []string{"application/json"}},
		Body:      []byte(`{"model":"gpt-5","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
	})
	require.NoError(t, err)

	converted, err := inbound.TransformStream(ctx, llmStream)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(projectID).
		SetChannelID(channelID).
		SetModelID("gpt-5").
		SetFormat(llm.APIFormatOpenAIChatCompletion.String()).
		SetStatus(request.StatusProcessing).
		SetStream(true).
		SetRequestBody([]byte(`{}`)). // store_request_body disabled: body is empty in production
		Save(ctx)
	require.NoError(t, err)

	persistentStream := NewInboundPersistentStream(
		ctx,
		converted,
		req,
		&ent.RequestExecution{ID: 1},
		state.RequestService,
		inbound,
		nil,
		state,
	)

	var events []*httpclient.StreamEvent
	for persistentStream.Next() {
		events = append(events, persistentStream.Current())
	}
	persistentStream.AcknowledgeDelivery()
	require.NoError(t, persistentStream.Close())

	return events, req
}

// TestSyntheticChain_UnterminatedFinalLineCompletesWithEmptyRequestBody is the
// production reproduction: the upstream ends the SSE body with an unterminated
// "data: [DONE]" and the persisted request body is empty. It asserts the
// client stream ends with data: [DONE] and the request persists as completed.
func TestSyntheticChain_UnterminatedFinalLineCompletesWithEmptyRequestBody(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	_, requestService, _, _ := setupTestServices(t, client)

	state := &PersistenceState{RequestService: requestService}

	events, req := driveSyntheticChain(t, client, ctx, project.ID, ch.ID, state)

	require.NotEmpty(t, events)
	require.Equal(t, "[DONE]", string(events[len(events)-1].Data), "client stream must end with data: [DONE]")

	dbReq, err := client.Request.Get(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCompleted, dbReq.Status)
}

// TestOutboundPersistentStream_UnterminatedFinalLineCompletesExecution drives
// the real decoder and real openai outbound transformer (outbound direction)
// with the captured bytes: the execution must persist completed, not canceled.
func TestOutboundPersistentStream_UnterminatedFinalLineCompletesExecution(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	project := createTestProject(t, ctx, client)
	ch := createTestChannel(t, ctx, client)
	_, requestService, _, usageLogService := setupTestServices(t, client)

	req, err := client.Request.Create().
		SetProjectID(project.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-5").
		SetStatus(request.StatusProcessing).
		SetStream(true).
		SetRequestBody([]byte(`{}`)).
		Save(ctx)
	require.NoError(t, err)
	exec, err := client.RequestExecution.Create().
		SetRequestID(req.ID).
		SetProjectID(project.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-5").
		SetFormat(llm.APIFormatOpenAIChatCompletion.String()).
		SetStatus(requestexecution.StatusProcessing).
		SetStream(true).
		SetRequestBody([]byte(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	body, err := os.ReadFile(syntheticFixturePath())
	require.NoError(t, err)
	raw := httpclient.NewDefaultSSEDecoder(ctx, io.NopCloser(bytes.NewReader(body)))

	outbound, err := openai.NewOutboundTransformer("https://provider.test", "key")
	require.NoError(t, err)
	state := &PersistenceState{}
	persistent := NewOutboundPersistentStream(ctx, raw, req, exec, requestService, usageLogService, outbound, nil, state)

	var last *httpclient.StreamEvent
	for persistent.Next() {
		last = persistent.Current()
	}
	require.NoError(t, persistent.Close())
	require.Equal(t, "[DONE]", string(last.Data), "outbound stream must deliver [DONE]")

	dbExec, err := client.RequestExecution.Get(ctx, exec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCompleted, dbExec.Status, "outbound execution must be completed, not canceled")
}
