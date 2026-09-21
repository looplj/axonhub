package biz

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

type endpointDetectTransport struct {
	mu        sync.Mutex
	requests  []string
	responder func(path string) (*http.Response, error)
}

func (t *endpointDetectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.requests = append(t.requests, req.URL.Path)
	t.mu.Unlock()

	return t.responder(req.URL.Path)
}

func newEndpointDetectTestChannel(t *testing.T, ctx context.Context, client *ent.Client, baseURL string) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("detect").
		SetBaseURL(baseURL).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4o-mini"}).
		SetDefaultTestModel("gpt-4o-mini").
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func TestDetectChannelEndpoints_ClassifiesProtocols(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	transport := &endpointDetectTransport{
		responder: func(path string) (*http.Response, error) {
			status := http.StatusBadRequest
			switch {
			case strings.HasSuffix(path, "/chat/completions"):
				status = http.StatusNotFound
			case strings.HasSuffix(path, "/messages"):
				status = http.StatusUnauthorized
			}

			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"probe"}}`)),
			}, nil
		},
	}
	svc.httpClient = httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	ch := newEndpointDetectTestChannel(t, ctx, client, "https://upstream.example")

	payload, err := svc.DetectChannelEndpoints(ctx, DetectChannelEndpointsInput{
		ChannelID: objects.GUID{Type: "Channel", ID: ch.ID},
	})
	require.NoError(t, err)
	require.Len(t, payload.Endpoints, 3)

	byFormat := map[string]DetectedChannelEndpoint{}
	for _, ep := range payload.Endpoints {
		byFormat[ep.APIFormat] = ep
	}

	chat := byFormat["openai/chat_completions"]
	require.False(t, chat.Supported)
	require.Equal(t, http.StatusNotFound, chat.StatusCode)
	require.Equal(t, endpointDetectReasonNotFound, chat.Reason)

	responses := byFormat["openai/responses"]
	require.True(t, responses.Supported)
	require.Equal(t, http.StatusBadRequest, responses.StatusCode)
	require.Equal(t, endpointDetectReasonSupported, responses.Reason)

	anthropic := byFormat["anthropic/messages"]
	require.False(t, anthropic.Supported)
	require.Equal(t, http.StatusUnauthorized, anthropic.StatusCode)
	require.Equal(t, endpointDetectReasonAuthError, anthropic.Reason)

	require.Len(t, transport.requests, 3)
}

func TestDetectChannelEndpoints_TransportErrorIsUnreachable(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	transport := &endpointDetectTransport{
		responder: func(path string) (*http.Response, error) {
			if strings.HasSuffix(path, "/responses") {
				return nil, io.ErrUnexpectedEOF
			}

			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		},
	}
	svc.httpClient = httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	ch := newEndpointDetectTestChannel(t, ctx, client, "https://upstream.example")

	payload, err := svc.DetectChannelEndpoints(ctx, DetectChannelEndpointsInput{
		ChannelID: objects.GUID{Type: "Channel", ID: ch.ID},
	})
	require.NoError(t, err)

	byFormat := map[string]DetectedChannelEndpoint{}
	for _, ep := range payload.Endpoints {
		byFormat[ep.APIFormat] = ep
	}

	require.False(t, byFormat["openai/responses"].Supported)
	require.Equal(t, endpointDetectReasonUnreachable, byFormat["openai/responses"].Reason)
	require.True(t, byFormat["openai/chat_completions"].Supported)
}

func TestResolveEndpointDetectModel_PrefersRequestThenDefaults(t *testing.T) {
	requested := " request-model "
	entity := &ent.Channel{
		DefaultTestModel: "default-model",
		SupportedModels:  []string{"supported-model"},
	}
	require.Equal(t, "request-model", resolveEndpointDetectModel(entity, &requested))
	require.Equal(t, "default-model", resolveEndpointDetectModel(entity, nil))

	entity.DefaultTestModel = ""
	require.Equal(t, "supported-model", resolveEndpointDetectModel(entity, nil))

	entity.SupportedModels = nil
	require.Equal(t, "", resolveEndpointDetectModel(entity, nil))
}
