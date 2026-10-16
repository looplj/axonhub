package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type deliveryCapabilitiesStream struct {
	streams.Stream[*httpclient.StreamEvent]

	source   streams.Stream[*httpclient.StreamEvent]
	observer streams.DeliveryAcknowledger
}

// preserveDeliveryCapabilities keeps transport acknowledgements and finalization
// connected to the source when a decorator implements only the base stream API.
func preserveDeliveryCapabilities(source, adapted streams.Stream[*httpclient.StreamEvent]) streams.Stream[*httpclient.StreamEvent] {
	observer, ok := source.(streams.DeliveryAcknowledger)
	if !ok {
		return adapted
	}
	return &deliveryCapabilitiesStream{Stream: adapted, source: source, observer: observer}
}

func (s *deliveryCapabilitiesStream) AcknowledgeDelivery() {
	s.observer.AcknowledgeDelivery()
}

func (s *deliveryCapabilitiesStream) DeliveryAcknowledged() bool {
	return s.observer.DeliveryAcknowledged()
}

func (s *deliveryCapabilitiesStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	if boundary, ok := s.source.(streams.DeliveryBoundary); ok {
		return boundary.Deliver(ctx, write)
	}
	terminal, err := write()
	if err == nil && terminal {
		s.AcknowledgeDelivery()
	}
	return err
}

func (s *deliveryCapabilitiesStream) WaitForFinalization() {
	if waiter, ok := s.source.(streams.FinalizationWaiter); ok {
		waiter.WaitForFinalization()
	}
}

func (s *deliveryCapabilitiesStream) ExpectedStreamChoices() int {
	if choices, ok := s.source.(interface{ ExpectedStreamChoices() int }); ok {
		return choices.ExpectedStreamChoices()
	}
	return 0
}
