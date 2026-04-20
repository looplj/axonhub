package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// rateLimitExhaustedScore is the penalty score for channels that have exhausted their rate limits
// or are in cooldown. Must exceed the maximum possible positive score sum from all other strategies
// (currently ~1530: Trace=1000 + Error=200 + WeightRR=150 + Latency=80 + RateLimit=100)
// so that exhausted channels always rank last, while still remaining as fallback candidates.
const rateLimitExhaustedScore = -10000

// RateLimitProvider bundles the dependencies needed by RateLimitAwareStrategy.
type RateLimitProvider struct {
	RequestTracker    *ChannelRequestTracker
	ConnectionTracker ConnectionTracker
	ModelConnTracker  ModelConnectionTrackerInterface
	CostTracker       *ChannelCostTracker
	QuotaService      *biz.QuotaService
	rpmFlight         *singleflight.Group
	tpmFlight         *singleflight.Group
	costFlight        *singleflight.Group
}

// RateLimitAwareStrategy adjusts channel scores based on configured RPM/TPM rate limits and concurrency limits.
// Channels that have exhausted their rate limits receive a heavily negative score to be ranked last.
type RateLimitAwareStrategy struct {
	provider RateLimitProvider
	maxScore float64
}

// NewRateLimitAwareStrategy creates a new rate limit aware load balancing strategy.
func NewRateLimitAwareStrategy(provider RateLimitProvider) *RateLimitAwareStrategy {
	if provider.rpmFlight == nil {
		provider.rpmFlight = &singleflight.Group{}
	}
	if provider.tpmFlight == nil {
		provider.tpmFlight = &singleflight.Group{}
	}
	if provider.costFlight == nil {
		provider.costFlight = &singleflight.Group{}
	}

	return &RateLimitAwareStrategy{
		provider: provider,
		maxScore: 100.0,
	}
}

// resolveRPM returns the current RPM count, falling back to DB when the in-memory counter returns 0
// (e.g. after process restart or window rotation). When a DB fallback occurs, the in-memory counter
// is seeded so subsequent requests use the fast path.

func anchorKey(anchor *time.Time) string {
	if anchor == nil {
		return "nil"
	}
	return anchor.UTC().Format(time.RFC3339Nano)
}
func (s *RateLimitAwareStrategy) resolveRPM(ctx context.Context, channel *biz.Channel, rl *objects.ChannelRateLimit) int64 {
	rpmDuration := rl.GetRPMDuration().Duration()

	if s.provider.QuotaService == nil {
		rpm := s.provider.RequestTracker.GetRequestCountForDuration(channel.ID, rpmDuration, rl.RPMWindowAnchor)
		return rpm
	}

	if s.provider.RequestTracker.IsRequestWindowDbQueried(channel.ID, rpmDuration, rl.RPMWindowAnchor) {
		rpm := s.provider.RequestTracker.GetRequestCountForDuration(channel.ID, rpmDuration, rl.RPMWindowAnchor)
		return rpm
	}

	windowStart := objects.ComputeWindowStart(time.Now(), rpmDuration, rl.RPMWindowAnchor)
	windowEnd := windowStart.Add(rpmDuration)

	key := fmt.Sprintf("rpm:%d:%s:%s:%d", channel.ID, rpmDuration.String(), anchorKey(rl.RPMWindowAnchor), windowStart.Unix())
	v, sfErr, _ := s.provider.rpmFlight.Do(key, func() (any, error) {
		dbCount, err := s.provider.QuotaService.GetChannelRequestCountAllSources(ctx, channel.ID, biz.QuotaWindow{Start: &windowStart, End: &windowEnd})

		if err != nil {
			return nil, err
		}

		s.provider.RequestTracker.MarkRequestWindowDbQueried(channel.ID, rpmDuration, rl.RPMWindowAnchor)

		if dbCount <= 0 {
			return nil, nil
		}

		s.provider.RequestTracker.SeedRequestCountForDuration(channel.ID, dbCount, rpmDuration, rl.RPMWindowAnchor)
		return dbCount, nil
	})

	if sfErr != nil {
		log.Warn(ctx, "failed to fetch channel RPM from quota service",
			log.Int("channel_id", channel.ID),
			log.Cause(sfErr),
		)
	}

	if dbCount, ok := v.(int64); ok {
		return dbCount
	}

	rpm := s.provider.RequestTracker.GetRequestCountForDuration(channel.ID, rpmDuration, rl.RPMWindowAnchor)
	return rpm
}

// resolveTPM returns the current TPM count, falling back to DB when the in-memory counter returns 0.
// Same seeding logic as resolveRPM.
func (s *RateLimitAwareStrategy) resolveTPM(ctx context.Context, channel *biz.Channel, rl *objects.ChannelRateLimit) int64 {
	tpmDuration := rl.GetTPMDuration().Duration()

	if s.provider.QuotaService == nil {
		tpm := s.provider.RequestTracker.GetTokenCountForDuration(channel.ID, tpmDuration, rl.TPMWindowAnchor)
		return tpm
	}

	if s.provider.RequestTracker.IsTokenWindowDbQueried(channel.ID, tpmDuration, rl.TPMWindowAnchor) {
		tpm := s.provider.RequestTracker.GetTokenCountForDuration(channel.ID, tpmDuration, rl.TPMWindowAnchor)
		return tpm
	}

	windowStart := objects.ComputeWindowStart(time.Now(), tpmDuration, rl.TPMWindowAnchor)
	windowEnd := windowStart.Add(tpmDuration)

	key := fmt.Sprintf("tpm:%d:%s:%s:%d", channel.ID, tpmDuration.String(), anchorKey(rl.TPMWindowAnchor), windowStart.Unix())
	v, sfErr, _ := s.provider.tpmFlight.Do(key, func() (any, error) {
		dbCount, err := s.provider.QuotaService.GetChannelTokenCountAllSources(ctx, channel.ID, biz.QuotaWindow{Start: &windowStart, End: &windowEnd})

		if err != nil {
			return nil, err
		}

		s.provider.RequestTracker.MarkTokenWindowDbQueried(channel.ID, tpmDuration, rl.TPMWindowAnchor)

		if dbCount <= 0 {
			return nil, nil
		}

		s.provider.RequestTracker.SeedTokenCountForDuration(channel.ID, dbCount, tpmDuration, rl.TPMWindowAnchor)
		return dbCount, nil
	})

	if sfErr != nil {
		log.Warn(ctx, "failed to fetch channel TPM from quota service",
			log.Int("channel_id", channel.ID),
			log.Cause(sfErr),
		)
	}

	if dbCount, ok := v.(int64); ok {
		return dbCount
	}

	tpm := s.provider.RequestTracker.GetTokenCountForDuration(channel.ID, tpmDuration, rl.TPMWindowAnchor)
	return tpm
}

// Name returns the strategy name.
func (s *RateLimitAwareStrategy) Name() string {
	return "RateLimitAware"
}

func (s *RateLimitAwareStrategy) resolveConcurrencyLimit(ctx context.Context, channel *biz.Channel) (limit int64, source string, configured bool) {
	if channel.Settings != nil && channel.Settings.RateLimit != nil {
		if rl := channel.Settings.RateLimit; rl.MaxConcurrent != nil && *rl.MaxConcurrent > 0 {
			return *rl.MaxConcurrent, "rate_limit_config", true
		}
	}

	if s.provider.ConnectionTracker == nil {
		log.Debug(ctx, "connection tracker is nil; skipping concurrency limit check",
			log.Int("channel_id", channel.ID),
		)
		return 0, "", false
	}

	limit = int64(s.provider.ConnectionTracker.GetMaxConnections(channel.ID))
	if limit <= 0 {
		return 0, "", false
	}

	return limit, "connection_tracker_default", false
}

// Score calculates the score based on channel rate limit usage.
// This is the production path with minimal overhead.
func (s *RateLimitAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	if s.provider.RequestTracker == nil {
		return s.maxScore
	}

	// Check if channel is in cooldown (429 Retry-After)
	if s.provider.RequestTracker.IsCoolingDown(channel.ID) {
		return rateLimitExhaustedScore
	}

	settings := channel.Settings
	if settings == nil || settings.RateLimit == nil {
		if s.provider.ConnectionTracker != nil {
			if concurrencyLimit, _, _ := s.resolveConcurrencyLimit(ctx, channel); concurrencyLimit > 0 {
				concurrent := s.provider.ConnectionTracker.GetActiveConnections(channel.ID)
				if int64(concurrent) >= concurrencyLimit {
					return rateLimitExhaustedScore
				}

				ratio := float64(concurrent) / float64(concurrencyLimit)

				score := s.maxScore * (1 - ratio)
				if score < 0 {
					score = 0
				}

				return score
			}
		}

		return s.maxScore
	}

	rl := settings.RateLimit

	var maxRatio float64

	// Check RPM (Requests Per Minute)
	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.resolveRPM(ctx, channel, rl)
		if rpm >= *rl.RPM {
			return rateLimitExhaustedScore
		}

		ratio := float64(rpm) / float64(*rl.RPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	// Check TPM (Tokens Per Minute)
	if rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.resolveTPM(ctx, channel, rl)
		if tpm >= *rl.TPM {
			return rateLimitExhaustedScore
		}

		ratio := float64(tpm) / float64(*rl.TPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	if rl.Cost != nil && rl.Cost.IsPositive() {
		currentCost, costCached := decimal.Zero, false

		if s.provider.CostTracker != nil {
			currentCost, costCached = s.provider.CostTracker.GetCachedCost(channel.ID)
		}

		if !costCached {
			if s.provider.QuotaService == nil {
				log.Warn(ctx, "cost limit configured but no cached cost data and no quota service for fallback",
					log.Int("channel_id", channel.ID),
				)
				return s.maxScore * 0.5
			}
		}

		if !costCached && s.provider.QuotaService != nil {
			costDuration := rl.GetCostDuration()
			windowStart := objects.ComputeWindowStart(time.Now(), costDuration.Duration(), rl.CostWindowAnchor)
			windowEnd := windowStart.Add(costDuration.Duration())

			key := fmt.Sprintf("cost:%d:%s:%s:%d", channel.ID, costDuration.Duration().String(), anchorKey(rl.CostWindowAnchor), windowStart.Unix())
			v, sfErr, _ := s.provider.costFlight.Do(key, func() (any, error) {
				fetchedCost, err := s.provider.QuotaService.GetChannelCost(ctx, channel.ID, biz.QuotaWindow{Start: &windowStart, End: &windowEnd})
				if err != nil {
					return nil, err
				}

				cost := decimal.NewFromFloat(fetchedCost)

				if s.provider.CostTracker != nil {
					s.provider.CostTracker.SetCachedCost(channel.ID, cost, windowEnd)
				}

				return cost, nil
			})

			if cost, ok := v.(decimal.Decimal); ok {
				currentCost = cost
				costCached = true
			} else if sfErr != nil {
				log.Warn(ctx, "failed to fetch channel cost from quota service",
					log.Int("channel_id", channel.ID),
					log.Cause(sfErr),
				)
			}
		}

		if costCached && currentCost.GreaterThanOrEqual(*rl.Cost) {
			return rateLimitExhaustedScore
		}

		if costCached {
			ratio := currentCost.Div(*rl.Cost).InexactFloat64()
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	if s.provider.ConnectionTracker != nil {
		if concurrencyLimit, _, _ := s.resolveConcurrencyLimit(ctx, channel); concurrencyLimit > 0 {
			concurrent := s.provider.ConnectionTracker.GetActiveConnections(channel.ID)
			if int64(concurrent) >= concurrencyLimit {
				return rateLimitExhaustedScore
			}

			ratio := float64(concurrent) / float64(concurrencyLimit)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	if s.provider.ModelConnTracker != nil && ctx != nil {
		modelID := requestedModelFromContext(ctx)
		if modelID != "" {
			// Per-model concurrent limits are only enforced when explicitly configured in ModelConcurrent.
			// When no per-model limit is configured (hasCustom=false), the channel-wide MaxConcurrent check
			// (via connectionTracker above) handles the fallback.
			// Example: MaxConcurrent=5 with ModelConcurrent={"gpt-4":2} allows gpt-4 up to 2 concurrent,
			// while other models share the channel-wide 5 concurrent limit.
			if modelLimit, hasCustom := rl.GetModelConcurrentLimit(modelID); hasCustom && modelLimit > 0 {
				modelConcurrent := int64(s.provider.ModelConnTracker.GetModelConnectionCount(channel.ID, modelID))
				if modelConcurrent >= modelLimit {
					return rateLimitExhaustedScore
				}

				ratio := float64(modelConcurrent) / float64(modelLimit)
				if ratio > maxRatio {
					maxRatio = ratio
				}
			}
		}
	}

	score := s.maxScore * (1 - maxRatio)
	if score < 0 {
		score = 0
	}

	return score
}

// ScoreWithDebug calculates the score with detailed debug information.
func (s *RateLimitAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	if s.provider.RequestTracker == nil {
		return s.maxScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        s.maxScore,
			Details:      map[string]any{"reason": "no_request_tracker"},
		}
	}

	startTime := time.Now()

	details := map[string]any{
		"channel_id": channel.ID,
	}

	// Check if channel is in cooldown (429 Retry-After)
	if until, ok := s.provider.RequestTracker.GetCooldownUntil(channel.ID); ok {
		score := float64(rateLimitExhaustedScore)
		details["reason"] = "channel_in_cooldown"
		details["exhausted"] = true
		details["cooldown_until"] = until.Format(time.RFC3339)

		return score, StrategyScore{
			StrategyName: s.Name(),
			Score:        score,
			Details:      details,
			Duration:     time.Since(startTime),
		}
	}

	settings := channel.Settings

	if settings == nil || settings.RateLimit == nil {
		if concurrencyLimit, source, _ := s.resolveConcurrencyLimit(ctx, channel); concurrencyLimit > 0 && s.provider.ConnectionTracker != nil {
			concurrent := s.provider.ConnectionTracker.GetActiveConnections(channel.ID)
			details["concurrent_limit"] = concurrencyLimit
			details["concurrent_current"] = concurrent
			details["concurrency_limit_source"] = source

			if int64(concurrent) >= concurrencyLimit {
				score := float64(rateLimitExhaustedScore)
				details["concurrent_exhausted"] = true
				details["exhausted"] = true
				details["score"] = score

				return score, StrategyScore{
					StrategyName: s.Name(),
					Score:        score,
					Details:      details,
					Duration:     time.Since(startTime),
				}
			}

			maxRatio := float64(concurrent) / float64(concurrencyLimit)

			score := s.maxScore * (1 - maxRatio)
			if score < 0 {
				score = 0
			}

			details["max_ratio"] = maxRatio
			details["score"] = score
			details["reason"] = "default_connection_limit_fallback"

			return score, StrategyScore{
				StrategyName: s.Name(),
				Score:        score,
				Details:      details,
				Duration:     time.Since(startTime),
			}
		}

		score := s.maxScore
		details["reason"] = "no_rate_limit_configured"

		return score, StrategyScore{
			StrategyName: s.Name(),
			Score:        score,
			Details:      details,
			Duration:     time.Since(startTime),
		}
	}

	rl := settings.RateLimit

	var maxRatio float64

	exhausted := false

	// Check RPM
	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.resolveRPM(ctx, channel, rl)
		details["rpm_limit"] = *rl.RPM
		details["rpm_current"] = rpm
		details["rpm_duration"] = string(rl.GetRPMDuration())

		if rpm >= *rl.RPM {
			exhausted = true
			details["rpm_exhausted"] = true
		} else {
			ratio := float64(rpm) / float64(*rl.RPM)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	// Check TPM
	if !exhausted && rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.resolveTPM(ctx, channel, rl)
		details["tpm_limit"] = *rl.TPM
		details["tpm_current"] = tpm
		details["tpm_duration"] = string(rl.GetTPMDuration())

		if tpm >= *rl.TPM {
			exhausted = true
			details["tpm_exhausted"] = true
		} else {
			ratio := float64(tpm) / float64(*rl.TPM)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	// Check Cost
	if !exhausted && rl.Cost != nil && rl.Cost.IsPositive() {
		currentCost, costCached := decimal.Zero, false

		if s.provider.CostTracker != nil {
			currentCost, costCached = s.provider.CostTracker.GetCachedCost(channel.ID)
		}

		if !costCached {
			if s.provider.QuotaService == nil {
				log.Warn(ctx, "cost limit configured but no cached cost data and no quota service for fallback",
					log.Int("channel_id", channel.ID),
				)
				details["cost_check_skipped"] = true
				score := s.maxScore * 0.5
				return score, StrategyScore{
					StrategyName: s.Name(),
					Score:        score,
					Details:      details,
					Duration:     time.Since(startTime),
				}
			}
		}

		if !costCached && s.provider.QuotaService != nil {
			costDuration := rl.GetCostDuration()
			windowStart := objects.ComputeWindowStart(time.Now(), costDuration.Duration(), rl.CostWindowAnchor)
			windowEnd := windowStart.Add(costDuration.Duration())

			key := fmt.Sprintf("cost:%d:%s:%s:%d", channel.ID, costDuration.Duration().String(), anchorKey(rl.CostWindowAnchor), windowStart.Unix())
			v, sfErr, _ := s.provider.costFlight.Do(key, func() (any, error) {
				fetchedCost, err := s.provider.QuotaService.GetChannelCost(ctx, channel.ID, biz.QuotaWindow{Start: &windowStart, End: &windowEnd})
				if err != nil {
					return nil, err
				}

				cost := decimal.NewFromFloat(fetchedCost)

				if s.provider.CostTracker != nil {
					s.provider.CostTracker.SetCachedCost(channel.ID, cost, windowEnd)
				}

				return cost, nil
			})

			if cost, ok := v.(decimal.Decimal); ok {
				currentCost = cost
				costCached = true
				details["cost_fetched_from_db"] = true
			} else if sfErr != nil {
				details["cost_fetch_error"] = sfErr.Error()
			}
		}

		if costCached {
			details["cost_limit"] = rl.Cost.String()
			details["cost_current"] = currentCost.String()
			details["cost_duration"] = string(rl.GetCostDuration())

			if currentCost.GreaterThanOrEqual(*rl.Cost) {
				exhausted = true
				details["cost_exhausted"] = true
			} else {
				ratio := currentCost.Div(*rl.Cost).InexactFloat64()
				if ratio > maxRatio {
					maxRatio = ratio
				}
			}
		}
	}

	// Check concurrent requests using explicit MaxConcurrent first, then default tracker fallback.
	if !exhausted && s.provider.ConnectionTracker != nil {
		if concurrencyLimit, source, configured := s.resolveConcurrencyLimit(ctx, channel); concurrencyLimit > 0 {
			concurrent := s.provider.ConnectionTracker.GetActiveConnections(channel.ID)
			details["concurrent_limit"] = concurrencyLimit
			details["concurrent_current"] = concurrent
			details["concurrency_limit_source"] = source
			details["concurrent_limit_configured"] = configured

			if int64(concurrent) >= concurrencyLimit {
				exhausted = true
				details["concurrent_exhausted"] = true
			} else {
				ratio := float64(concurrent) / float64(concurrencyLimit)
				if ratio > maxRatio {
					maxRatio = ratio
				}
			}
		}
	}

	if !exhausted && s.provider.ModelConnTracker != nil && ctx != nil {
		modelID := requestedModelFromContext(ctx)
		if modelID != "" {
			// Per-model concurrent limits are only enforced when explicitly configured in ModelConcurrent.
			// When no per-model limit is configured (hasCustom=false), the channel-wide MaxConcurrent check
			// (via connectionTracker above) handles the fallback.
			// Example: MaxConcurrent=5 with ModelConcurrent={"gpt-4":2} allows gpt-4 up to 2 concurrent,
			// while other models share the channel-wide 5 concurrent limit.
			if modelLimit, hasCustom := rl.GetModelConcurrentLimit(modelID); hasCustom && modelLimit > 0 {
				modelConcurrent := int64(s.provider.ModelConnTracker.GetModelConnectionCount(channel.ID, modelID))
				details["model_concurrent_limit"] = modelLimit
				details["model_concurrent_current"] = modelConcurrent
				details["model_concurrent_model"] = modelID

				if modelConcurrent >= modelLimit {
					exhausted = true
					details["model_concurrent_exhausted"] = true
				} else {
					ratio := float64(modelConcurrent) / float64(modelLimit)
					if ratio > maxRatio {
						maxRatio = ratio
					}
				}
			}
		}
	}

	var score float64
	if exhausted {
		score = rateLimitExhaustedScore
		details["exhausted"] = true
	} else {
		score = s.maxScore * (1 - maxRatio)
		if score < 0 {
			score = 0
		}
	}

	details["max_ratio"] = maxRatio
	details["score"] = score

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "RateLimitAwareStrategy: scoring",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("score", score),
			log.Any("details", details),
		)
	}

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
		Duration:     time.Since(startTime),
	}
}
