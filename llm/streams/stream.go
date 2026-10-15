package streams

import "context"

// Stream represents a generic stream interface
// The caller should check the Err() method to ensure there's no error.
type Stream[T any] interface {
	// Next indicate if there's a next item.
	Next() bool
	// Current returns the current event
	Current() T
	// Err returns any error that occurred
	Err() error
	// Close closes the stream
	Close() error
}

// DeliveryAcknowledger is an optional stream capability. A transport calls
// AcknowledgeDelivery only after the semantic terminal boundary was written
// successfully; generation and accounting remain separate concerns.
type DeliveryAcknowledger interface {
	AcknowledgeDelivery()
	DeliveryAcknowledged() bool
}

type DeliveryBoundary interface {
	Deliver(context.Context, func() (bool, error)) error
}

// FinalizationWaiter waits for source-owned stream accounting to finish.
// Stream Close remains nonblocking for client transport callers.
type FinalizationWaiter interface {
	WaitForFinalization()
}
