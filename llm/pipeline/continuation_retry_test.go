package pipeline

import (
	"context"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResponsesRecoveryStopsBothRetryBranches(t *testing.T) {
	// Given
	retries, switches := 0, 0
	outbound := &mockOutbound{
		transformRequest: func(context.Context, *llm.Request) (*httpclient.Request, error) {
			return nil, shared.ResponsesRecoveryError()
		},
		canRetry:        func(error) bool { return true },
		prepareForRetry: func(context.Context) error { retries++; return nil },
		hasMoreChannels: func() bool { return true },
		nextChannel:     func(context.Context) error { switches++; return nil },
	}
	p := &pipeline{Inbound: &mockInbound{}, Outbound: outbound, maxChannelRetries: 2, maxSameChannelRetries: 2}
	// When
	_, err := p.Process(t.Context(), &httpclient.Request{})
	// Then
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	require.Zero(t, retries)
	require.Zero(t, switches)
}
