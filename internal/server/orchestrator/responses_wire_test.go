package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesCodexHTTPCompleteHistoryExpandsBeforeDispatch(t *testing.T) {
	// Given: a real Codex HTTP finalizer and production pipeline against a local wire listener.
	frames := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		frames <- body
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"id":"provider_next","object":"response","status":"completed","model":"gpt-5","output":[]}`))
		if err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	ctx := shared.WithResponsesAttempt(shared.WithSessionScope(t.Context(), "api_key:1:project:1"), &shared.ResponsesAttempt{})
	store := newResponsesSessionStore()
	toolOutput := strings.Repeat("x", 374*(1<<20)/100)
	root := []byte(`{"model":"gpt-5","input":[{"role":"user","content":"` + strings.Repeat("a", 200<<10) + `"},{"type":"function_call_output","call_id":"call_canary","output":"` + toolOutput + `"}]}`)
	store.record(ctx, root, []byte(`{"id":"provider_previous","status":"completed","output":[{"id":"item_canary","type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}]}`))
	clientBody := []byte(`{"model":"gpt-5","previous_response_id":"provider_previous","input":"followup"}`)
	p, pipe := responsesCodexWirePipeline(t, server, store, clientBody)
	p.state.ChannelModelsCandidates[0].Channel.Settings.PassThroughBody = lo.ToPtr(false)
	// When
	_, err := pipe.Process(ctx, &httpclient.Request{Body: clientBody, JSONBody: clientBody, Headers: make(http.Header)})
	// Then
	require.NoError(t, err)
	sent := <-frames
	require.Greater(t, len(sent), 389*(1<<20)/100)
	require.False(t, gjson.GetBytes(sent, "previous_response_id").Exists())
	require.Equal(t, "call_canary", gjson.GetBytes(sent, "input.1.call_id").String())
	require.Equal(t, toolOutput, gjson.GetBytes(sent, "input.1.output").String())
	require.Equal(t, "item_canary", gjson.GetBytes(sent, "input.2.id").String())
	require.Equal(t, clientBody, p.state.RawRequest.JSONBody)
}

func TestResponsesCodexHTTPOverBudgetFailsBeforeDispatch(t *testing.T) {
	for _, previous := range []string{"over_budget", "unknown"} {
		t.Run(previous, func(t *testing.T) { responsesCodexHTTPUnavailableWire(t, previous) })
	}
}

func responsesCodexHTTPUnavailableWire(t *testing.T, previous string) {
	t.Helper()
	// Given
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	ctx := shared.WithResponsesAttempt(shared.WithSessionScope(t.Context(), "api_key:1:project:1"), &shared.ResponsesAttempt{})
	store := newResponsesSessionStore()
	store.record(ctx, []byte(`{"input":"`+strings.Repeat("x", 16<<20)+`"}`), []byte(`{"id":"over_budget","status":"completed","output":[]}`))
	body := fmt.Appendf(nil, `{"model":"gpt-5","previous_response_id":%q,"input":"delta"}`, previous)
	_, pipe := responsesCodexWirePipeline(t, server, store, body)
	// When
	_, err := pipe.Process(ctx, &httpclient.Request{Body: body, Headers: make(http.Header)})
	// Then
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	require.Zero(t, writes.Load())
}

type responsesWireProcessor interface {
	Process(context.Context, *httpclient.Request) (*pipeline.Result, error)
}

func responsesCodexWirePipeline(t *testing.T, server *httptest.Server, store *responsesSessionStore, body []byte) (*PersistentOutboundTransformer, responsesWireProcessor) {
	t.Helper()
	outbound, err := codex.NewOutboundTransformer(codex.Params{BaseURL: server.URL + "#", TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "synthetic"})})
	require.NoError(t, err)
	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "synthetic", Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}}, Outbound: outbound}
	candidate := &ChannelModelsCandidate{Channel: channel, APIFormat: "openai/responses", Models: []biz.ChannelModelEntry{{RequestModel: "gpt-5", ActualModel: "gpt-5"}}}
	state := &PersistenceState{ChannelModelsCandidates: []*ChannelModelsCandidate{candidate}, ResponsesContinuation: newResponsesContinuation(store, body)}
	inbound, persistent := NewPersistentTransformers(state, responses.NewInboundTransformer())
	pipe := pipeline.NewFactory(httpclient.NewHttpClientWithClient(server.Client())).Pipeline(inbound, persistent, pipeline.WithRetry(2, 2, 0), pipeline.WithMiddlewares(applyPassThroughRequestBody(persistent, nil), applyResponsesContinuationBody(persistent), verifyResponsesAttempt(persistent), finalizeTransportRequest(persistent), captureResponsesProvider(persistent)))
	return persistent, pipe
}
