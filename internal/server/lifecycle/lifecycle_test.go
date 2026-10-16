package lifecycle

import (
	"context"
	"testing"
)

func TestManagerShutdownPropagatesThroughAttachedRequest(t *testing.T) {
	manager := New()
	request := manager.Attach(context.Background())
	manager.Shutdown(nil)

	select {
	case <-FromContext(request).Context().Done():
	default:
		t.Fatal("shutdown was not propagated")
	}
}
