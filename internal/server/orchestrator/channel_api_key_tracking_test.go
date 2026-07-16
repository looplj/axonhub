package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

type mockAPIKeyTransformer struct {
	mockTransformer
	apiKey string
}

func (m *mockAPIKeyTransformer) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	request, err := m.mockTransformer.TransformRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	request.Auth = &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: m.apiKey,
	}

	return request, nil
}

func TestPersistentOutboundTransformer_TransformRequest_CapturesChannelAPIKey(t *testing.T) {
	const apiKey = `channel-key`

	wrapped := &mockAPIKeyTransformer{apiKey: apiKey}
	channel := &biz.Channel{
		Channel:  &ent.Channel{ID: 1, Name: `test-channel`},
		Outbound: wrapped,
	}
	state := &PersistenceState{
		ChannelModelsCandidates: []*ChannelModelsCandidate{{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{RequestModel: `gpt-4`, ActualModel: `gpt-4`}},
		}},
	}
	processor := &PersistentOutboundTransformer{wrapped: wrapped, state: state}

	_, err := processor.TransformRequest(context.Background(), &llm.Request{Model: `gpt-4`})
	require.NoError(t, err)
	require.Equal(t, apiKey, state.ChannelAPIKey)
}

func TestPersistentOutboundTransformer_PrepareErrorFailover_ExcludesKeyWhenDisableFails(t *testing.T) {
	const (
		channelID = 42
		apiKey    = `failed-key`
	)

	client := enttest.NewEntClient(t, "sqlite3", "file:failover-disable-error?mode=memory&_fk=0")
	defer client.Close()

	ctx := contexts.Initialize(context.Background())
	channel := &biz.Channel{Channel: &ent.Channel{
		ID: channelID,
		Settings: &objects.ChannelSettings{APIKeyFailover: &objects.ChannelAPIKeyFailover{
			Enabled:     true,
			StatusCodes: []int{http.StatusPaymentRequired},
		}},
	}}
	state := &PersistenceState{
		ChannelService: biz.NewChannelServiceForTest(client),
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
		},
		ChannelAPIKey: apiKey,
	}
	processor := &PersistentOutboundTransformer{state: state}

	prepared, err := processor.PrepareErrorFailover(ctx, &httpclient.Error{StatusCode: http.StatusPaymentRequired})

	require.NoError(t, err)
	require.False(t, prepared)
	require.True(t, contexts.IsChannelAPIKeyExcluded(ctx, channelID, apiKey))
	require.True(t, processor.apiKeyFailoverExhausted)
}
