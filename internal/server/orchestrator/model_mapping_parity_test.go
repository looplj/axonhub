package orchestrator

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestModelMapping_SharedState(t *testing.T) {
	profile := &objects.APIKeyProfiles{
		ActiveProfile: "active",
		Profiles: []objects.APIKeyProfile{{
			Name: "active",
			ModelMappings: []objects.ModelMapping{
				{From: "client-alias", To: "routing-alias"},
			},
		}},
	}
	for _, tt := range []struct {
		name          string
		apiKey        *ent.APIKey
		model         string
		originalModel string
		wantModel     string
	}{
		{name: "nil key captures shared model", model: "client-alias", wantModel: "client-alias"},
		{name: "plain key", apiKey: &ent.APIKey{}, model: "client-alias", wantModel: "client-alias"},
		{name: "inactive profile", apiKey: &ent.APIKey{Profiles: &objects.APIKeyProfiles{Profiles: profile.Profiles}}, model: "client-alias", wantModel: "client-alias"},
		{name: "missing profile", apiKey: &ent.APIKey{Profiles: &objects.APIKeyProfiles{ActiveProfile: "missing", Profiles: profile.Profiles}}, model: "client-alias", wantModel: "client-alias"},
		{name: "active mapping", apiKey: &ent.APIKey{Profiles: profile}, model: "client-alias", wantModel: "routing-alias"},
		{name: "active nonmatch", apiKey: &ent.APIKey{Profiles: profile}, model: "other-alias", wantModel: "other-alias"},
		{name: "nil key restores routing model", model: "provider-model", originalModel: "routing-alias", wantModel: "routing-alias"},
		{name: "profile preserves routing model", apiKey: &ent.APIKey{Profiles: profile}, model: "client-alias", originalModel: "saved-alias", wantModel: "saved-alias"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given an optional profile and shared routing state.
			state := &PersistenceState{APIKey: tt.apiKey, ModelMapper: NewModelMapper(), OriginalModel: tt.originalModel}
			middleware := &apiKeyModelMappingMiddleware{inbound: &PersistentInboundTransformer{state: state}}
			request := &llm.Request{Model: tt.model}

			// When the inbound model mapping stage runs.
			got, err := middleware.OnInboundLlmRequest(t.Context(), request)

			// Then routing uses the post-profile model while responses retain the client model.
			require.NoError(t, err)
			require.Same(t, request, got)
			require.Equal(t, tt.wantModel, got.Model)
			require.Equal(t, tt.wantModel, state.OriginalModel)
			require.Equal(t, tt.model, middleware.RequestModel)
		})
	}
}

func TestModelMapping_RejectsEmptyModel(t *testing.T) {
	// Given an empty model and no API key.
	state := &PersistenceState{ModelMapper: NewModelMapper()}
	middleware := &apiKeyModelMappingMiddleware{inbound: &PersistentInboundTransformer{state: state}}
	// When the mapping stage validates the request.
	got, err := middleware.OnInboundLlmRequest(t.Context(), &llm.Request{})
	// Then no shared state is initialized for the invalid request.
	require.ErrorIs(t, err, biz.ErrInvalidModel)
	require.Nil(t, got)
	require.Empty(t, state.OriginalModel)
	require.Empty(t, middleware.RequestModel)
}

func TestModelMapping_PipelineConditionalOverrides(t *testing.T) {
	for _, tt := range []struct {
		name         string
		apiKey       *ent.APIKey
		clientModel  string
		routingModel string
		wantTier     string
	}{
		{name: "playground matching alias", clientModel: "fast-alias", routingModel: "fast-alias", wantTier: "priority"},
		{name: "playground unmatched alias", clientModel: "other-alias", routingModel: "other-alias", wantTier: "auto"},
		{name: "plain API key matching alias", apiKey: &ent.APIKey{}, clientModel: "fast-alias", routingModel: "fast-alias", wantTier: "priority"},
		{
			name: "profile mapping matches post-profile alias",
			apiKey: &ent.APIKey{Profiles: &objects.APIKeyProfiles{
				ActiveProfile: "active",
				Profiles:      []objects.APIKeyProfile{{Name: "active", ModelMappings: []objects.ModelMapping{{From: "client-alias", To: "fast-alias"}}}},
			}},
			clientModel: "client-alias", routingModel: "fast-alias", wantTier: "priority",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given real transformers, empty shared state, and a local synthetic provider.
			type providerRequest struct {
				body    []byte
				headers http.Header
			}
			captured := make(chan providerRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "cannot read request", http.StatusBadRequest)
					return
				}
				captured <- providerRequest{body: body, headers: r.Header.Clone()}
				w.Header().Set("Content-Type", "application/json")
				if _, err := io.WriteString(w, `{"id":"resp","model":"provider-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`); err != nil {
					return
				}
			}))
			t.Cleanup(server.Close)
			ctx, db, state := newUpstreamModelPersistenceTest(t)
			state.APIKey = tt.apiKey
			entity, err := db.Channel.Get(ctx, state.Request.ChannelID)
			require.NoError(t, err)
			condition := `{{ eq .RequestModel "fast-alias" }}`
			entity.Settings = &objects.ChannelSettings{
				BodyOverrideOperations: []objects.OverrideOperation{
					{Op: objects.OverrideOpSet, Path: "service_tier", Value: "auto"},
					{Op: objects.OverrideOpSet, Path: "service_tier", Value: "priority", Condition: condition},
				},
				HeaderOverrideOperations: []objects.OverrideOperation{
					{Op: objects.OverrideOpSet, Path: "X-Conditional-Alias", Value: "{{ .RequestModel }}", Condition: condition},
					{Op: objects.OverrideOpSet, Path: "X-Routing-Model", Value: "{{ .RequestModel }}"},
					{Op: objects.OverrideOpSet, Path: "X-Provider-Model", Value: "{{ .Model }}"},
				},
			}
			provider, err := openai.NewOutboundTransformer(server.URL, "synthetic-test-key")
			require.NoError(t, err)
			state.ChannelModelsCandidates = []*ChannelModelsCandidate{{
				Channel:   &biz.Channel{Channel: entity, Outbound: provider},
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Models:    []biz.ChannelModelEntry{{RequestModel: tt.routingModel, ActualModel: "provider-model", Source: "direct"}},
			}}
			inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer())
			pipe := pipeline.NewFactory(httpclient.NewHttpClientWithProxy(nil)).Pipeline(inbound, outbound,
				pipeline.WithMiddlewares(applyModelMapping(inbound), applyOverrideRequestBody(outbound), applyOverrideRequestHeaders(outbound)))

			// When a request traverses the real pipeline and provider HTTP transport.
			result, err := pipe.Process(ctx, buildTestRequest(tt.clientModel, "hello", false))

			// Then body/header conditions use the routing alias, independently of the provider model.
			require.NoError(t, err)
			observed := <-captured
			require.Equal(t, "provider-model", gjson.GetBytes(observed.body, "model").String())
			require.Equal(t, tt.wantTier, gjson.GetBytes(observed.body, "service_tier").String())
			wantConditional := ""
			if tt.wantTier == "priority" {
				wantConditional = "fast-alias"
			}
			require.Equal(t, wantConditional, observed.headers.Get("X-Conditional-Alias"))
			require.Equal(t, tt.routingModel, observed.headers.Get("X-Routing-Model"))
			require.Equal(t, "provider-model", observed.headers.Get("X-Provider-Model"))
			require.Equal(t, tt.clientModel, gjson.GetBytes(result.Response.Body, "model").String())
			t.Logf("provider model=provider-model service_tier=%s conditional_alias=%q routing_model=%s client_model=%s", tt.wantTier, wantConditional, tt.routingModel, tt.clientModel)
		})
	}
}
