package biz

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/log"
)

// HealthStatus represents the health status of a model.
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"
	StatusDegraded HealthStatus = "degraded"
	StatusDisabled HealthStatus = "disabled"
)

// ModelHealthPolicy defines the policy for model health management.
type ModelHealthPolicy struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	// [触发阈值]
	// 连续失败多少次触发降级 (建议: 3)
	DegradeThreshold int `json:"degrade_threshold" yaml:"degrade_threshold"`
	// 连续失败多少次触发熔断 (建议: 5)
	DisableThreshold int `json:"disable_threshold" yaml:"disable_threshold"`

	// [时间控制]
	// 错误计数有效期 (建议: 30m)。超过这个时间未发生新错误，计数器清零。
	FailureStatsTTL time.Duration `json:"failure_stats_ttl" yaml:"failure_stats_ttl"`
	// 熔断后，多久尝试一次探测 (建议: 5m)
	ProbeInterval time.Duration `json:"probe_interval" yaml:"probe_interval"`

	// [权重控制]
	// 降级状态下的权重系数 (建议: 0.3)
	DegradedWeight float64 `json:"degraded_weight" yaml:"degraded_weight"`
}

// DefaultModelHealthPolicy returns the default model health policy.
func DefaultModelHealthPolicy() *ModelHealthPolicy {
	return &ModelHealthPolicy{
		Enabled:          true,
		DegradeThreshold: 3,
		DisableThreshold: 5,
		FailureStatsTTL:  30 * time.Minute,
		ProbeInterval:    5 * time.Minute,
		DegradedWeight:   0.3,
	}
}

// Validate validates the model health policy.
func (p *ModelHealthPolicy) Validate() error {
	if p.DegradeThreshold >= p.DisableThreshold {
		return fmt.Errorf("degrade_threshold (%d) must be less than disable_threshold (%d)",
			p.DegradeThreshold, p.DisableThreshold)
	}

	if p.DegradedWeight < 0 || p.DegradedWeight > 1 {
		return fmt.Errorf("degraded_weight must be between 0 and 1, got %f", p.DegradedWeight)
	}

	return nil
}

// ModelHealthStats represents the runtime health statistics for a model.
type ModelHealthStats struct {
	sync.RWMutex // 读写锁保护

	// 标识
	ChannelID int
	ModelID   string

	// 当前状态
	Status HealthStatus

	// 计数器
	ConsecutiveFailures int       // 连续失败次数
	LastFailureAt       time.Time // 上次失败时间
	LastSuccessAt       time.Time // 上次成功时间

	// 恢复控制
	NextProbeAt time.Time // 下一次允许探测的时间 (用于 Disabled 状态)

	// 探测控制 (防止并发穿透)
	probingInProgress int32 // 使用 atomic 操作
	probeAttempts     int   // 探测尝试次数，用于指数退避
}

// ModelHealthManager manages the health status of models across channels.
type ModelHealthManager struct {
	systemService *SystemService

	// 内存中的模型健康统计
	statsMap map[string]*ModelHealthStats // key: "channelID:modelID"
	mutex    sync.RWMutex
}

// NewModelHealthManager creates a new model health manager.
func NewModelHealthManager(systemService *SystemService) *ModelHealthManager {
	return &ModelHealthManager{
		systemService: systemService,
		statsMap:      make(map[string]*ModelHealthStats),
	}
}

// getStatsKey generates a unique key for channel and model combination.
func (m *ModelHealthManager) getStatsKey(channelID int, modelID string) string {
	return fmt.Sprintf("%d:%s", channelID, modelID)
}

// getStats gets or creates model health stats for the given channel and model.
func (m *ModelHealthManager) getStats(channelID int, modelID string) *ModelHealthStats {
	key := m.getStatsKey(channelID, modelID)

	m.mutex.RLock()
	stats, exists := m.statsMap[key]
	m.mutex.RUnlock()

	if exists {
		return stats
	}

	// Create new stats if not exists
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Double check after acquiring write lock
	if stats, exists := m.statsMap[key]; exists {
		return stats
	}

	stats = &ModelHealthStats{
		ChannelID:           channelID,
		ModelID:             modelID,
		Status:              StatusHealthy,
		ConsecutiveFailures: 0,
		LastSuccessAt:       time.Now(),
	}

	m.statsMap[key] = stats
	return stats
}

// GetPolicy retrieves the model health policy from system settings.
func (m *ModelHealthManager) GetPolicy(ctx context.Context) *ModelHealthPolicy {
	// For now, return default policy
	// TODO: Integrate with system settings when ready
	return DefaultModelHealthPolicy()
}

// RecordError records an error for the specified channel and model.
func (m *ModelHealthManager) RecordError(ctx context.Context, channelID int, modelID string) {
	stats := m.getStats(channelID, modelID)
	stats.Lock()
	defer stats.Unlock()

	now := time.Now()
	policy := m.GetPolicy(ctx)

	if !policy.Enabled {
		return
	}

	// 1. TTL 检查：防止僵尸计数
	if stats.ConsecutiveFailures > 0 {
		if now.Sub(stats.LastFailureAt) > policy.FailureStatsTTL {
			log.Info(ctx, "Resetting expired failure stats for model",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Int("old_failures", stats.ConsecutiveFailures),
			)
			stats.ConsecutiveFailures = 0
		}
	}

	// 2. 更新计数
	stats.ConsecutiveFailures++
	stats.LastFailureAt = now

	// 3. 状态流转判断
	// 优先判断熔断，再判断降级
	if stats.ConsecutiveFailures >= policy.DisableThreshold {
		if stats.Status != StatusDisabled {
			stats.Status = StatusDisabled
			stats.NextProbeAt = now.Add(policy.ProbeInterval) // 设定下次探测时间
			stats.probeAttempts = 0                           // 重置探测计数

			log.Warn(ctx, "Model DISABLED due to consecutive failures",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Int("failures", stats.ConsecutiveFailures),
			)
		} else {
			// 如果已经是 Disabled，使用指数退避更新探测时间
			backoffMultiplier := math.Pow(2, float64(stats.probeAttempts))
			if backoffMultiplier > 8 { // 最大 8 倍
				backoffMultiplier = 8
			}

			nextInterval := time.Duration(float64(policy.ProbeInterval) * backoffMultiplier)
			stats.NextProbeAt = now.Add(nextInterval)
			stats.probeAttempts++

			log.Debug(ctx, "Updated probe time for disabled model",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Time("next_probe_at", stats.NextProbeAt),
				log.Int("probe_attempts", stats.probeAttempts),
			)
		}
	} else if stats.ConsecutiveFailures >= policy.DegradeThreshold {
		if stats.Status != StatusDegraded {
			stats.Status = StatusDegraded

			log.Warn(ctx, "Model DEGRADED due to consecutive failures",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Int("failures", stats.ConsecutiveFailures),
			)
		}
	}
}

// RecordSuccess records a successful request for the specified channel and model.
func (m *ModelHealthManager) RecordSuccess(ctx context.Context, channelID int, modelID string) {
	stats := m.getStats(channelID, modelID)
	stats.Lock()
	defer stats.Unlock()

	stats.LastSuccessAt = time.Now()

	// 只要成功一次，立即重置所有负面状态
	if stats.Status != StatusHealthy {
		log.Info(ctx, "Model RECOVERED to Healthy",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
			log.String("previous_status", string(stats.Status)),
			log.Int("previous_failures", stats.ConsecutiveFailures),
		)
	}

	stats.Status = StatusHealthy
	stats.ConsecutiveFailures = 0
	stats.NextProbeAt = time.Time{} // 清空探测时间
	stats.probeAttempts = 0         // 重置探测计数

	// 重置探测标记 (使用 atomic 操作)
	// atomic.StoreInt32(&stats.probingInProgress, 0)
}

// GetModelHealth returns the current health status of a model.
func (m *ModelHealthManager) GetModelHealth(ctx context.Context, channelID int, modelID string) *ModelHealthStats {
	stats := m.getStats(channelID, modelID)
	stats.RLock()
	defer stats.RUnlock()

	// Return a copy to avoid concurrent modification
	return &ModelHealthStats{
		ChannelID:           stats.ChannelID,
		ModelID:             stats.ModelID,
		Status:              stats.Status,
		ConsecutiveFailures: stats.ConsecutiveFailures,
		LastFailureAt:       stats.LastFailureAt,
		LastSuccessAt:       stats.LastSuccessAt,
		NextProbeAt:         stats.NextProbeAt,
		probeAttempts:       stats.probeAttempts,
	}
}

// GetEffectiveWeight calculates the effective weight for a model based on its health status.
func (m *ModelHealthManager) GetEffectiveWeight(ctx context.Context, channelID int, modelID string, baseWeight float64) float64 {
	stats := m.getStats(channelID, modelID)
	stats.RLock()
	defer stats.RUnlock()

	policy := m.GetPolicy(ctx)
	if !policy.Enabled {
		return baseWeight
	}

	switch stats.Status {
	case StatusHealthy:
		return baseWeight

	case StatusDegraded:
		// 降级：降低权重，减少流量，但保留探测能力
		return baseWeight * policy.DegradedWeight

	case StatusDisabled:
		// 熔断：默认权重为 0
		// 【关键】Lazy Probe 逻辑：
		// 如果当前时间已经超过了 NextProbeAt，说明可以放行一个请求去探测了
		if time.Now().After(stats.NextProbeAt) {
			// 返回一个极小的非零权重，让它有机会被选中
			// 实际生产中建议配合原子标记位，防止瞬间并发穿透，但简单场景下直接返回小权重也可
			return 0.01
		}
		return 0.0

	default:
		return baseWeight
	}
}

// GetAllUnhealthyModels returns all models that are not in healthy status.
func (m *ModelHealthManager) GetAllUnhealthyModels(ctx context.Context) []*ModelHealthStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var unhealthy []*ModelHealthStats
	for _, stats := range m.statsMap {
		stats.RLock()
		if stats.Status != StatusHealthy {
			// Create a copy
			unhealthy = append(unhealthy, &ModelHealthStats{
				ChannelID:           stats.ChannelID,
				ModelID:             stats.ModelID,
				Status:              stats.Status,
				ConsecutiveFailures: stats.ConsecutiveFailures,
				LastFailureAt:       stats.LastFailureAt,
				LastSuccessAt:       stats.LastSuccessAt,
				NextProbeAt:         stats.NextProbeAt,
				probeAttempts:       stats.probeAttempts,
			})
		}
		stats.RUnlock()
	}

	return unhealthy
}

// GetChannelModelHealth returns health status for all models in a specific channel.
func (m *ModelHealthManager) GetChannelModelHealth(ctx context.Context, channelID int) []*ModelHealthStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var channelModels []*ModelHealthStats
	for _, stats := range m.statsMap {
		stats.RLock()
		if stats.ChannelID == channelID {
			// Create a copy
			channelModels = append(channelModels, &ModelHealthStats{
				ChannelID:           stats.ChannelID,
				ModelID:             stats.ModelID,
				Status:              stats.Status,
				ConsecutiveFailures: stats.ConsecutiveFailures,
				LastFailureAt:       stats.LastFailureAt,
				LastSuccessAt:       stats.LastSuccessAt,
				NextProbeAt:         stats.NextProbeAt,
				probeAttempts:       stats.probeAttempts,
			})
		}
		stats.RUnlock()
	}

	return channelModels
}

// ResetModelStatus manually resets a model's health status to healthy.
// This is useful for manual intervention by operators.
func (m *ModelHealthManager) ResetModelStatus(ctx context.Context, channelID int, modelID string) error {
	stats := m.getStats(channelID, modelID)
	stats.Lock()
	defer stats.Unlock()

	oldStatus := stats.Status
	stats.Status = StatusHealthy
	stats.ConsecutiveFailures = 0
	stats.NextProbeAt = time.Time{}
	stats.probeAttempts = 0

	log.Info(ctx, "Model status manually reset to healthy",
		log.Int("channel_id", channelID),
		log.String("model_id", modelID),
		log.String("previous_status", string(oldStatus)),
	)

	return nil
}
