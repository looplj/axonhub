package api

import (
	"context"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func writeStreamDelivery(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent], write func() (bool, error)) error {
	if boundary, ok := stream.(streams.DeliveryBoundary); ok {
		return boundary.Deliver(ctx, write)
	}
	terminal, err := write()
	if err == nil && terminal {
		if observer, ok := stream.(streams.DeliveryAcknowledger); ok {
			observer.AcknowledgeDelivery()
		}
	}
	return err
}

func acknowledgeStreamDelivery(_ context.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	if observer, ok := stream.(streams.DeliveryAcknowledger); ok {
		observer.AcknowledgeDelivery()
	}
}

func (s *primedStream) AcknowledgeDelivery() {
	if observer, ok := s.stream.(streams.DeliveryAcknowledger); ok {
		observer.AcknowledgeDelivery()
	}
}

func (s *primedStream) DeliveryAcknowledged() bool {
	observer, ok := s.stream.(streams.DeliveryAcknowledger)
	return ok && observer.DeliveryAcknowledged()
}

func (s *primedStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	boundary, ok := s.stream.(streams.DeliveryBoundary)
	if !ok {
		terminal, err := write()
		if err == nil && terminal {
			s.AcknowledgeDelivery()
		}
		return err
	}
	return boundary.Deliver(ctx, write)
}

func (s *primedStream) WaitForFinalization() {
	if waiter, ok := s.stream.(streams.FinalizationWaiter); ok {
		waiter.WaitForFinalization()
	}
}

func (s *upstreamErrorStream) AcknowledgeDelivery() {
	if observer, ok := s.stream.(streams.DeliveryAcknowledger); ok {
		observer.AcknowledgeDelivery()
	}
}

func (s *upstreamErrorStream) DeliveryAcknowledged() bool {
	observer, ok := s.stream.(streams.DeliveryAcknowledger)
	return ok && observer.DeliveryAcknowledged()
}

func (s *upstreamErrorStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	boundary, ok := s.stream.(streams.DeliveryBoundary)
	if !ok {
		terminal, err := write()
		if err == nil && terminal {
			s.AcknowledgeDelivery()
		}
		return err
	}
	return boundary.Deliver(ctx, write)
}

func (s *upstreamErrorStream) WaitForFinalization() {
	if waiter, ok := s.stream.(streams.FinalizationWaiter); ok {
		waiter.WaitForFinalization()
	}
}

type deliveryStream struct {
	streams.Stream[*httpclient.StreamEvent]
	streams.DeliveryAcknowledger

	boundary        streams.DeliveryBoundary
	waiter          streams.FinalizationWaiter
	expectedChoices int
}

func (s *deliveryStream) ExpectedStreamChoices() int { return s.expectedChoices }

func (s *deliveryStream) WaitForFinalization() {
	if s.waiter != nil {
		s.waiter.WaitForFinalization()
	}
}

func preserveStreamDelivery(adapted, source streams.Stream[*httpclient.StreamEvent]) streams.Stream[*httpclient.StreamEvent] {
	if observer, ok := source.(streams.DeliveryAcknowledger); ok {
		boundary, _ := source.(streams.DeliveryBoundary)
		waiter, _ := source.(streams.FinalizationWaiter)
		return &deliveryStream{Stream: adapted, DeliveryAcknowledger: observer, boundary: boundary, waiter: waiter, expectedChoices: streamExpectedChoices(source)}
	}
	return adapted
}

func (s *deliveryStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	if s.boundary == nil {
		terminal, err := write()
		if err == nil && terminal {
			s.AcknowledgeDelivery()
		}
		return err
	}
	return s.boundary.Deliver(ctx, write)
}
