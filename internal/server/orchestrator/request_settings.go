package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// RequestSettings is the snapshot of system settings one chat request uses
// from start to finish. Process loads it once, so every stage of the request
// sees the same values even if an administrator changes a setting mid-request.
// Treat it as read-only.
type RequestSettings struct {
	RetryPolicy          biz.RetryPolicy
	StoragePolicy        biz.StoragePolicy
	QuotaRouting         biz.QuotaRoutingSettings
	ModelSettings        biz.SystemModelSettings
	PassThrough          bool
	UserAgentPassThrough bool
	InjectUsageCost      bool
}

// loadRequestSettings reads each setting once. A setting that fails to load
// falls back to the same default its readers used before the snapshot existed.
func (processor *ChatCompletionOrchestrator) loadRequestSettings(ctx context.Context) *RequestSettings {
	systemService := processor.SystemService

	settings := &RequestSettings{
		RetryPolicy:   *systemService.RetryPolicyOrDefault(ctx),
		StoragePolicy: *systemService.StoragePolicyOrDefault(ctx),
		QuotaRouting:  systemService.QuotaRoutingSettingsOrDefault(ctx),
		ModelSettings: *systemService.ModelSettingsOrDefault(ctx),
	}

	var err error
	if settings.PassThrough, err = systemService.PassThrough(ctx); err != nil {
		log.Warn(ctx, "failed to get global pass-through setting", log.Cause(err))
	}

	if settings.UserAgentPassThrough, err = systemService.UserAgentPassThrough(ctx); err != nil {
		log.Warn(ctx, "failed to get global user agent pass through setting", log.Cause(err))
	}

	if settings.InjectUsageCost, err = systemService.InjectUsageCostEnabled(ctx); err != nil {
		log.Warn(ctx, "failed to get inject usage cost setting", log.Cause(err))
	}

	return settings
}

// noRequestSettings stands in for states built without a snapshot, such as in
// unit tests: every global switch reads as off.
var noRequestSettings = &RequestSettings{}

// settings returns the request's settings snapshot.
func (s *PersistenceState) settings() *RequestSettings {
	if s == nil || s.Settings == nil {
		return noRequestSettings
	}

	return s.Settings
}

type requestSettingsContextKey struct{}

// withRequestSettings lets components shared across requests, such as load
// balancers and the default selector, read the snapshot of the request they
// are serving.
func withRequestSettings(ctx context.Context, settings *RequestSettings) context.Context {
	return context.WithValue(ctx, requestSettingsContextKey{}, settings)
}

func requestSettingsFromContext(ctx context.Context) *RequestSettings {
	settings, _ := ctx.Value(requestSettingsContextKey{}).(*RequestSettings)
	return settings
}
