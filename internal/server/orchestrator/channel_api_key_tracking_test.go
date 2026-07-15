package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
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
