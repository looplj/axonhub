package biz

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/metrics"
)

// RegisterMetricResourceInfo installs metadata only when metrics export is
// enabled. No per-request database lookups or exported credentials are needed.
func RegisterMetricResourceInfo(lc fx.Lifecycle, client *ent.Client, cfg metrics.Config, provider *sdkmetric.MeterProvider) {
	if !cfg.Enabled {
		return
	}
	svc := &metricResourceInfo{client: client}
	var registration metric.Registration
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var err error
			registration, err = metrics.RegisterResourceInfo(provider.Meter("axonhub"), svc.snapshot)
			return err
		},
		OnStop: func(context.Context) error {
			if registration != nil {
				return registration.Unregister()
			}
			return nil
		},
	})
}

// Cache for one minute, independently of the five-second export interval.
// Only successful snapshots are cached; a failed read must not renew old names.
type metricResourceInfo struct {
	client   *ent.Client
	mu       sync.Mutex
	loadedAt time.Time
	names    metrics.ResourceNames
}

func (s *metricResourceInfo) snapshot(ctx context.Context) (metrics.ResourceNames, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loadedAt.IsZero() && time.Since(s.loadedAt) < time.Minute {
		return s.names, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	names, err := authz.RunWithSystemBypass(ctx, "export-metric-display-names", func(ctx context.Context) (metrics.ResourceNames, error) {
		return s.load(ctx)
	})
	if err != nil {
		return metrics.ResourceNames{}, err
	}
	s.names, s.loadedAt = names, time.Now()
	return names, nil
}

func (s *metricResourceInfo) load(ctx context.Context) (metrics.ResourceNames, error) {
	var result metrics.ResourceNames
	channels, err := s.client.Channel.Query().Select(channel.FieldID, channel.FieldName).All(ctx)
	if err != nil {
		return result, fmt.Errorf("load channel metric names: %w", err)
	}
	for _, item := range channels {
		result.Channels = append(result.Channels, metrics.ResourceName{ID: item.ID, Name: item.Name})
	}
	projects, err := s.client.Project.Query().Select(project.FieldID, project.FieldName).All(ctx)
	if err != nil {
		return result, fmt.Errorf("load project metric names: %w", err)
	}
	for _, item := range projects {
		result.Projects = append(result.Projects, metrics.ResourceName{ID: item.ID, Name: item.Name})
	}
	keys, err := s.client.APIKey.Query().Select(apikey.FieldID, apikey.FieldName).All(ctx)
	if err != nil {
		return result, fmt.Errorf("load API key metric names: %w", err)
	}
	for _, item := range keys {
		result.APIKeys = append(result.APIKeys, metrics.ResourceName{ID: item.ID, Name: item.Name})
	}
	users, err := s.client.User.Query().Select(user.FieldID, user.FieldFirstName, user.FieldLastName).All(ctx)
	if err != nil {
		return result, fmt.Errorf("load user metric names: %w", err)
	}
	for _, item := range users {
		name := strings.TrimSpace(item.FirstName + " " + item.LastName)
		if name == "" {
			name = fmt.Sprintf("User %d", item.ID)
		}
		result.Users = append(result.Users, metrics.ResourceName{ID: item.ID, Name: name})
	}
	return result, nil
}
