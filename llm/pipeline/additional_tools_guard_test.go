package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestPipeline_Process_UnsupportedAdditionalToolsDoesNotRetry(t *testing.T) {
	prepareCalls := 0
	switchCalls := 0
	executorCalls := 0
	unsupported := errors.Join(transformer.ErrUnsupportedConversion, transformer.ErrInvalidRequest)

	inbound := &mockInbound{transformRequest: func(context.Context, *httpclient.Request) (*llm.Request, error) {
		return &llm.Request{}, nil
	}}
	outbound := &mockOutbound{
		transformRequest: func(context.Context, *llm.Request) (*httpclient.Request, error) {
			return nil, unsupported
		},
		canRetry: func(error) bool {
			return true
		},
		prepareForRetry: func(context.Context) error {
			prepareCalls++
			return nil
		},
		hasMoreChannels: func() bool {
			return true
		},
		nextChannel: func(context.Context) error {
			switchCalls++
			return nil
		},
	}
	executor := &mockExecutor{do: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
		executorCalls++
		return &httpclient.Response{}, nil
	}}

	pipe := NewFactory(executor).Pipeline(inbound, outbound, WithRetry(2, 2, 0))
	_, err := pipe.Process(context.Background(), &httpclient.Request{})

	require.Error(t, err)
	require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Zero(t, prepareCalls)
	require.Zero(t, switchCalls)
	require.Zero(t, executorCalls)
}

func TestPipeline_Process_PreservesOrdinaryInvalidRequestRetry(t *testing.T) {
	transformCalls := 0
	prepareCalls := 0
	executorCalls := 0
	outbound := &mockOutbound{
		transformRequest: func(context.Context, *llm.Request) (*httpclient.Request, error) {
			transformCalls++
			if transformCalls == 1 {
				return nil, fmt.Errorf("malformed request: %w", transformer.ErrInvalidRequest)
			}

			return &httpclient.Request{}, nil
		},
		canRetry: func(error) bool { return true },
		prepareForRetry: func(context.Context) error {
			prepareCalls++
			return nil
		},
	}
	executor := &mockExecutor{do: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
		executorCalls++
		return &httpclient.Response{}, nil
	}}

	pipe := NewFactory(executor).Pipeline(&mockInbound{}, outbound, WithRetry(1, 1, 0))
	_, err := pipe.Process(context.Background(), &httpclient.Request{})

	require.NoError(t, err)
	require.Equal(t, 2, transformCalls)
	require.Equal(t, 1, prepareCalls)
	require.Equal(t, 1, executorCalls)
}

func TestPipeline_Process_LaterUnsupportedConversionStopsRetries(t *testing.T) {
	transformCalls := 0
	prepareCalls := 0
	switchCalls := 0
	executorCalls := 0
	unsupported := errors.Join(transformer.ErrUnsupportedConversion, transformer.ErrInvalidRequest)
	outbound := &mockOutbound{
		transformRequest: func(context.Context, *llm.Request) (*httpclient.Request, error) {
			transformCalls++
			if transformCalls == 1 {
				return nil, errors.New("temporary conversion failure")
			}

			return nil, unsupported
		},
		canRetry: func(error) bool { return true },
		prepareForRetry: func(context.Context) error {
			prepareCalls++
			return nil
		},
		hasMoreChannels: func() bool { return true },
		nextChannel: func(context.Context) error {
			switchCalls++
			return nil
		},
	}
	executor := &mockExecutor{do: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
		executorCalls++
		return &httpclient.Response{}, nil
	}}

	pipe := NewFactory(executor).Pipeline(&mockInbound{}, outbound, WithRetry(1, 1, 0))
	_, err := pipe.Process(context.Background(), &httpclient.Request{})

	require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
	require.Equal(t, 2, transformCalls)
	require.Equal(t, 1, prepareCalls)
	require.Zero(t, switchCalls)
	require.Zero(t, executorCalls)
}
