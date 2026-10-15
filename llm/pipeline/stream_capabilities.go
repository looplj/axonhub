package pipeline

import (
	"context"

	"github.com/looplj/axonhub/llm/streams"
)

type streamCapabilities[T any] struct {
	streams.Stream[T]

	observer       streams.DeliveryAcknowledger
	boundary       streams.DeliveryBoundary
	waiter         streams.FinalizationWaiter
	expectedChoice func() int
}

func (s *streamCapabilities[T]) AcknowledgeDelivery() {
	s.observer.AcknowledgeDelivery()
}

func (s *streamCapabilities[T]) DeliveryAcknowledged() bool {
	return s.observer.DeliveryAcknowledged()
}

func (s *streamCapabilities[T]) WaitForFinalization() {
	if s.waiter != nil {
		s.waiter.WaitForFinalization()
	}
}

func (s *streamCapabilities[T]) Deliver(ctx context.Context, write func() (bool, error)) error {
	if s.boundary != nil {
		return s.boundary.Deliver(ctx, write)
	}
	terminal, err := write()
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

	boundary, _ := source.(streams.DeliveryBoundary)
	waiter, _ := source.(streams.FinalizationWaiter)
	return &streamCapabilities[R]{Stream: adapted, observer: observer, boundary: boundary, waiter: waiter, expectedChoice: expected}
}
