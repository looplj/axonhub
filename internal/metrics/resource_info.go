package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

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
	return meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		names, err := load(ctx)
		if err != nil {
			// Display metadata must not prevent request telemetry from exporting
			// when the database is temporarily unavailable.
			log.Warn(ctx, "Failed to collect metric resource names", log.Cause(err))
			return nil
		}
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
}
