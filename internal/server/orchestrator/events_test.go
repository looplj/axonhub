package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestNextChannelFailoverEvent_NilBusAndNoSubscriber(t *testing.T) {
	t.Run("nil bus", func(t *testing.T) {
		processor := newFailoverTransformer(nil)
		require.NoError(t, processor.NextChannel(context.Background()))
		require.Equal(t, 1, processor.state.CurrentCandidateIndex)
		require.Equal(t, "beta", processor.GetCurrentChannel().Name)
	})

	t.Run("no subscriber", func(t *testing.T) {
		bus := events.NewBus()
		t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
		processor := newFailoverTransformer(bus)

		require.NoError(t, processor.NextChannel(context.Background()))

		require.Equal(t, 1, processor.state.CurrentCandidateIndex)
		require.Equal(t, "beta", processor.GetCurrentChannel().Name)
		require.Equal(t, int64(0), bus.Stats().Published)
		require.False(t, bus.HasSubscribers(events.TypeRequestFailover))
	})
}

func TestNextChannelFailoverEvent_SubscriberGetsLeftChannel(t *testing.T) {
	secret := "sk-failover-left-secret-value"
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	received := subscribeOrchestratorEvent(t, bus, events.TypeRequestFailover)
	processor := newFailoverTransformer(bus)
	processor.state.ChannelModelsCandidates[0].Channel.Credentials.APIKeys = []string{secret}

	require.NoError(t, processor.NextChannel(context.Background()))

	event := requireOrchestratorEvent(t, received)
	require.Equal(t, events.TypeRequestFailover, event.Type)
	require.Equal(t, events.SeverityWarning, event.Severity)
	require.NotEmpty(t, event.ID)
	require.Equal(t, time.UTC, event.OccurredAt.Location())
	require.Empty(t, event.Origin)
	require.Equal(t, 0, event.Depth)
	require.Nil(t, event.Subject.Credential)
	require.Equal(t, 1, event.Subject.Channel.ID)
	require.Equal(t, "alpha", event.Subject.Channel.Name)
	require.Equal(t, channel.TypeOpenai.String(), event.Subject.Channel.Type)
	require.Equal(t, "https://alpha.example", event.Subject.Channel.BaseURL)
	require.Equal(t, channel.StatusEnabled.String(), event.Subject.Channel.Status)
	require.Equal(t, "gpt-5", event.Subject.Model)
	require.Equal(t, 9, event.Subject.APIKey.ID)
	require.Equal(t, "gateway", event.Subject.APIKey.Name)
	require.Equal(t, 4, event.Subject.Project.ID)
	require.Equal(t, 12, event.Subject.Request.ID)
	require.Equal(t, 2, event.Data["toChannelId"])
	require.Equal(t, "beta", event.Data["toChannelName"])
	require.Equal(t, 3, event.Data["attempt"])
	require.Equal(t, "beta", processor.GetCurrentChannel().Name)
	requireNoOrchestratorSecret(t, event, secret)
}

func TestNextChannelFailoverEvent_FailedSwitchDoesNotPublish(t *testing.T) {
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	subscribeOrchestratorEvent(t, bus, events.TypeRequestFailover)
	left := failoverChannel(1, "alpha")
	processor := &PersistentOutboundTransformer{state: &PersistenceState{
		EventBus:              bus,
		OutboundAttempt:       1,
		CurrentCandidateIndex: 0,
		CurrentCandidate:      &ChannelModelsCandidate{Channel: left, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}},
		ChannelModelsCandidates: []*ChannelModelsCandidate{
			{Channel: left, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}}},
		},
	}}

	require.Error(t, processor.NextChannel(context.Background()))
	require.Equal(t, int64(0), bus.Stats().Published)
}

func TestRequestAttemptFailedEvent_PublishesMaskedCredential(t *testing.T) {
	secret := "sk-live-attempt-failed-secret-value"
	message := strings.Repeat("m", 600)
	ctx, client, state := newUpstreamModelPersistenceTest(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	received := subscribeOrchestratorEvent(t, bus, events.TypeRequestAttemptFailed)

	state.EventBus = bus
	state.OutboundAttempt = 2
	state.APIKey = &ent.APIKey{ID: 7, Name: "caller", ProjectID: state.Request.ProjectID}
	state.OriginalModel = "gpt-5"
	state.CurrentModelIndex = 0
	state.CurrentCandidate = &ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{
			ID:          11,
			Name:        "upstream",
			Type:        channel.TypeOpenai,
			BaseURL:     "https://up.example",
			Status:      channel.StatusEnabled,
			Credentials: objects.ChannelCredentials{APIKeys: []string{secret}},
		}},
		Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}},
	}
	state.RequestExec = createUpstreamModelTestExecution(t, ctx, client, state.Request, "gpt-5", llm.APIFormatOpenAIChatCompletion, false)
	middleware := &persistRequestExecutionMiddleware{outbound: &PersistentOutboundTransformer{state: state}}

	attemptCtx := contexts.WithChannelAPIKey(ctx, secret)
	middleware.OnOutboundRawError(attemptCtx, &llm.ResponseError{
		StatusCode: 429,
		Detail: llm.ErrorDetail{
			Type:    "rate_limit_error",
			Message: message,
		},
	})

	saved, err := client.RequestExecution.Get(ctx, state.RequestExec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusFailed, saved.Status)

	event := requireOrchestratorEvent(t, received)
	require.Equal(t, events.TypeRequestAttemptFailed, event.Type)
	require.Equal(t, events.SeverityWarning, event.Severity)
	require.NotEmpty(t, event.ID)
	require.Equal(t, time.UTC, event.OccurredAt.Location())
	require.Empty(t, event.Origin)
	require.Equal(t, 0, event.Depth)
	require.Equal(t, 11, event.Subject.Channel.ID)
	require.Equal(t, "upstream", event.Subject.Channel.Name)
	require.Equal(t, "gpt-5", event.Subject.Model)
	require.Equal(t, 7, event.Subject.APIKey.ID)
	require.Equal(t, "caller", event.Subject.APIKey.Name)
	require.Equal(t, state.Request.ProjectID, event.Subject.Project.ID)
	require.Equal(t, state.Request.ID, event.Subject.Request.ID)
	require.Equal(t, events.Fingerprint(secret), event.Subject.Credential.Fingerprint)
	require.Equal(t, events.Mask(secret), event.Subject.Credential.Masked)
	require.Equal(t, 2, event.Data["attempt"])
	require.Equal(t, 429, event.Data["statusCode"])
	require.Equal(t, "rate_limit_error", event.Data["errorType"])
	require.Equal(t, message[:512], event.Data["errorMessage"])
	require.LessOrEqual(t, len(event.Data["errorMessage"].(string)), 512)
	requireNoOrchestratorSecret(t, event, secret)
}

func TestRequestAttemptFailedEvent_OAuthDoesNotHashToken(t *testing.T) {
	token := "super-secret-oauth-access-token"
	ctx, client, state := newUpstreamModelPersistenceTest(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	received := subscribeOrchestratorEvent(t, bus, events.TypeRequestAttemptFailed)
	state.EventBus = bus
	state.OutboundAttempt = 1
	state.CurrentModelIndex = 0
	state.CurrentCandidate = &ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{
			ID:     15,
			Name:   "codex",
			Type:   channel.TypeCodex,
			Status: channel.StatusEnabled,
			Credentials: objects.ChannelCredentials{
				OAuth: &objects.OAuthCredentials{AccessToken: token, RefreshToken: "refresh-token-secret"},
			},
		}},
		Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5"}},
	}
	state.RequestExec = createUpstreamModelTestExecution(t, ctx, client, state.Request, "gpt-5", llm.APIFormatOpenAIChatCompletion, false)
	middleware := &persistRequestExecutionMiddleware{outbound: &PersistentOutboundTransformer{state: state}}

	middleware.OnOutboundRawError(contexts.WithChannelAPIKey(ctx, token), &llm.ResponseError{
		StatusCode: 401,
		Detail:     llm.ErrorDetail{Type: "authentication_error", Message: "unauthorized"},
	})

	event := requireOrchestratorEvent(t, received)
	require.Equal(t, events.OAuthFingerprint, event.Subject.Credential.Fingerprint)
	require.Equal(t, events.OAuthFingerprint, event.Subject.Credential.Masked)
	require.NotEqual(t, events.Fingerprint(token), event.Subject.Credential.Fingerprint)
	requireNoOrchestratorSecret(t, event, token)
	requireNoOrchestratorSecret(t, event, "refresh-token-secret")
}

func TestRequestAttemptFailedEvent_NilBusStillPersists(t *testing.T) {
	ctx, client, state := newUpstreamModelPersistenceTest(t)
	state.EventBus = nil
	state.RequestExec = createUpstreamModelTestExecution(t, ctx, client, state.Request, "gpt-5", llm.APIFormatOpenAIChatCompletion, false)
	middleware := &persistRequestExecutionMiddleware{outbound: &PersistentOutboundTransformer{state: state}}

	middleware.OnOutboundRawError(ctx, &llm.ResponseError{
		StatusCode: 500,
		Detail:     llm.ErrorDetail{Message: "boom"},
	})

	saved, err := client.RequestExecution.Get(ctx, state.RequestExec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusFailed, saved.Status)
}

func newFailoverTransformer(bus *events.Bus) *PersistentOutboundTransformer {
	left := failoverChannel(1, "alpha")
	right := failoverChannel(2, "beta")
	leftCandidate := &ChannelModelsCandidate{
		Channel: left,
		Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-5"}},
	}
	return &PersistentOutboundTransformer{state: &PersistenceState{
		EventBus:              bus,
		OutboundAttempt:       3,
		OriginalModel:         "gpt-5",
		CurrentCandidateIndex: 0,
		CurrentModelIndex:     0,
		CurrentCandidate:      leftCandidate,
		ChannelModelsCandidates: []*ChannelModelsCandidate{
			leftCandidate,
			{Channel: right, Models: []biz.ChannelModelEntry{{ActualModel: "gpt-5-beta"}}},
		},
		APIKey:  &ent.APIKey{ID: 9, Name: "gateway", ProjectID: 4},
		Request: &ent.Request{ID: 12, ProjectID: 4, ModelID: "gpt-5"},
	}}
}

func failoverChannel(id int, name string) *biz.Channel {
	return &biz.Channel{Channel: &ent.Channel{
		ID:      id,
		Name:    name,
		Type:    channel.TypeOpenai,
		BaseURL: "https://" + name + ".example",
		Status:  channel.StatusEnabled,
	}}
}

func subscribeOrchestratorEvent(t *testing.T, bus *events.Bus, eventType string) <-chan events.Event {
	t.Helper()
	received := make(chan events.Event, 4)
	bus.Subscribe("test-"+eventType, []string{eventType}, func(_ context.Context, event events.Event) {
		received <- event
	})
	return received
}

func requireOrchestratorEvent(t *testing.T, received <-chan events.Event) events.Event {
	t.Helper()
	select {
	case event := <-received:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for plugin event")
		return events.Event{}
	}
}

func requireNoOrchestratorSecret(t *testing.T, event events.Event, secret string) {
	t.Helper()
	encoded, err := json.Marshal(event.Map())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), secret)
	require.NotContains(t, fmt.Sprintf("%#v", event), secret)
}

func TestStreamFailureEvents(t *testing.T) {
	for _, tt := range []struct {
		name      string
		err       error
		event     *httpclient.StreamEvent
		completed bool
		message   string
	}{
		{name: "transport error", err: io.ErrUnexpectedEOF, message: "unexpected EOF"},
		{name: "silent EOF", message: ErrStreamIncomplete.Error()},
		{name: "provider failure", event: &httpclient.StreamEvent{Type: "response.failed", Data: []byte(`{"type":"response.failed","response":{"status":"failed","error":{"message":"provider failed"}}}`)}, message: "provider failed"},
		{name: "provider incomplete", event: &httpclient.StreamEvent{Type: "response.incomplete", Data: []byte(`{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`)}, message: "max_output_tokens"},
		{name: "completed despite trailing error", err: io.ErrUnexpectedEOF, event: &httpclient.StreamEvent{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"status":"completed"}}`)}, completed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := &mockStream{err: tt.err}
			if tt.event != nil {
				source.events = []*httpclient.StreamEvent{tt.event}
			}
			stream, client, _, state := newInboundPersistentStreamHelper(t, source, &mockInboundTransformer{})
			defer client.Close()
			bus := events.NewBus()
			defer bus.Close(context.Background())
			state.EventBus = bus
			received := subscribeOrchestratorEvent(t, bus, events.TypeRequestFailed)
			for stream.Next() {
				stream.Current()
			}
			if tt.completed {
				stream.AcknowledgeDelivery()
			}
			require.NoError(t, stream.Close())
			require.NoError(t, stream.Close())
			if tt.completed {
				require.Zero(t, bus.Stats().Published)
			} else {
				event := requireOrchestratorEvent(t, received)
				require.Equal(t, tt.message, event.Data["errorMessage"])
				// Process may also report an error after an internal stream close.
				publishRequestFailed(context.Background(), state, io.ErrUnexpectedEOF)
				require.Equal(t, int64(1), bus.Stats().Published)
			}
		})
	}
}

func TestAPIKeyQuotaExceededEvent(t *testing.T) {
	for _, persisted := range []bool{false, true} {
		t.Run(fmt.Sprintf("persisted=%t", persisted), func(t *testing.T) {
			bus := events.NewBus()
			defer bus.Close(context.Background())
			received := subscribeOrchestratorEvent(t, bus, events.TypeAPIKeyQuotaExceeded)
			state := &PersistenceState{
				EventBus: bus,
				APIKey:   &ent.APIKey{ID: 12, Name: "test-key", ProjectID: 34},
			}
			if persisted {
				state.Request = &ent.Request{ID: 56}
			}
			inbound := &PersistentInboundTransformer{state: state}
			inbound.publishAPIKeyQuotaExceeded(context.Background(), &llm.Request{Model: "test-model"}, "default", biz.QuotaWindow{})
			event := requireOrchestratorEvent(t, received)
			require.Equal(t, "test-model", event.Subject.Model)
			require.Equal(t, &events.APIKeyRef{ID: 12, Name: "test-key"}, event.Subject.APIKey)
			require.Equal(t, &events.ProjectRef{ID: 34}, event.Subject.Project)
			require.Equal(t, "default", event.Data["profile"])
			require.Equal(t, quotaWindowData(biz.QuotaWindow{}), event.Data["window"])
			if persisted {
				require.Equal(t, &events.RequestRef{ID: 56}, event.Subject.Request)
			} else {
				require.Nil(t, event.Subject.Request)
			}
			inbound.publishAPIKeyQuotaExceeded(context.Background(), nil, "default", biz.QuotaWindow{})
			require.Empty(t, requireOrchestratorEvent(t, received).Subject.Model)
		})
	}
	t.Run("missing state or bus", func(t *testing.T) {
		for _, inbound := range []*PersistentInboundTransformer{nil, {}, {state: &PersistenceState{}}} {
			require.NotPanics(t, func() {
				inbound.publishAPIKeyQuotaExceeded(context.Background(), nil, "default", biz.QuotaWindow{})
			})
		}
	})
}
