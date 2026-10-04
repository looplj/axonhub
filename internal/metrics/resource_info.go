package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/log"
)

// ResourceName is display metadata, kept separate from cumulative usage series
// so renaming a resource does not reset its counters or merge same-name IDs.
type ResourceName struct {
	ID   int
	Name string
}

type ResourceNames struct {
	Channels []ResourceName
	Projects []ResourceName
	APIKeys  []ResourceName
	Users    []ResourceName
}

var resourceInfoRefreshInterval = time.Minute

func RegisterResourceInfo(meter metric.Meter, load func(context.Context) (ResourceNames, error)) (metric.Registration, error) {
	types := []string{"channel", "project", "api_key", "user"}
	gauges := make([]metric.Int64ObservableGauge, len(types))
	instruments := make([]metric.Observable, len(types))
	for i, kind := range types {
		gauge, err := meter.Int64ObservableGauge("axonhub_"+kind+"_info", metric.WithDescription("Current display name for an AxonHub "+kind))
		if err != nil {
			return nil, fmt.Errorf("register %s metadata: %w", kind, err)
		}
		gauges[i], instruments[i] = gauge, gauge
	}
	var snapshot struct {
		sync.RWMutex
		names ResourceNames
	}
	refresh := func() {
		defer func() {
			if cause := recover(); cause != nil {
				log.Warn(context.Background(), "Metric resource refresh panicked", zap.Any("cause", cause))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		names, err := load(ctx)
		if err != nil {
			log.Warn(ctx, "Failed to refresh metric resource names", log.Cause(err))
			return
		}
		snapshot.Lock()
		snapshot.names = names
		snapshot.Unlock()
	}
	// Perform the initial load during registration, outside the observable
	// callback. Subsequent refreshes happen in the background.
	refresh()
	stop := make(chan struct{})
	go func() {
		defer func() {
			if cause := recover(); cause != nil {
				log.Warn(context.Background(), "Metric resource refresh loop panicked", zap.Any("cause", cause))
			}
		}()
		ticker := time.NewTicker(resourceInfoRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh()
			case <-stop:
				return
			}
		}
	}()
	registration, err := meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		snapshot.RLock()
		names := snapshot.names
		snapshot.RUnlock()
		groups := [][]ResourceName{names.Channels, names.Projects, names.APIKeys, names.Users}
		for i, group := range groups {
			for _, resource := range group {
				observer.ObserveInt64(gauges[i], 1, metric.WithAttributes(
					attribute.Int(types[i]+"_id", resource.ID),
					attribute.String(types[i]+"_name", resource.Name),
				))
			}
		}
		return nil
	}, instruments...)
	if err != nil {
		close(stop)
		return nil, err
	}
	return &resourceInfoRegistration{registration: registration, stop: stop}, nil
}

type resourceInfoRegistration struct {
	embedded.Registration
	registration metric.Registration
	stop         chan struct{}
}

func (r *resourceInfoRegistration) Unregister() error {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
	return r.registration.Unregister()
}
