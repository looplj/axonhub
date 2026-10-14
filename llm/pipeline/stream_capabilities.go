package pipeline

import (
	"context"

	"github.com/looplj/axonhub/llm/streams"
)

type streamCapabilities[T any] struct {
	streams.Stream[T]
	observer       streams.DeliveryAcknowledger
	expectedChoice func() int
}

func (s *streamCapabilities[T]) AcknowledgeDelivery() {
	s.observer.AcknowledgeDelivery()
}

func (s *streamCapabilities[T]) DeliveryAcknowledged() bool {
	return s.observer.DeliveryAcknowledged()
}

func (s *streamCapabilities[T]) Deliver(ctx context.Context, write func() (bool, error)) error {
	boundary, ok := s.Stream.(streams.DeliveryBoundary)
	if ok {
		return boundary.Deliver(ctx, write)
	}
	terminal, err := write()
	if err == nil && terminal && ctx.Err() != nil {
		return nil
	}
	if err == nil && terminal {
		s.AcknowledgeDelivery()
	}
	return err
}

func (s *streamCapabilities[T]) ExpectedStreamChoices() int {
	if s.expectedChoice == nil {
		return 0
	}
	return s.expectedChoice()
}

func preserveStreamCapabilities[T, R any](source streams.Stream[T], adapted streams.Stream[R]) streams.Stream[R] {
	observer, ok := source.(streams.DeliveryAcknowledger)
	if !ok {
		return adapted
	}

	var expected func() int
	if choices, ok := source.(interface{ ExpectedStreamChoices() int }); ok {
		expected = choices.ExpectedStreamChoices
	}

	return &streamCapabilities[R]{Stream: adapted, observer: observer, expectedChoice: expected}
}
