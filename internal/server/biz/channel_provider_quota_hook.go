// 职责: 定义渠道服务与提供商额度服务之间的可选回调契约。
// 关联文件:
//
//	internal/server/biz/channel_apikey_quota.go
//	internal/server/biz/provider_quota.go
//	internal/server/biz/fx_module.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	回调由额度服务在持有自身互斥锁时调用，实现方不得回调 ProviderQuotaService。
package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

// ChannelProviderQuotaInvalidator lets ChannelService discard quota state when
// a channel's provider identity changes. It lives in biz to avoid coupling the
// channel service to a concrete quota implementation.
type ChannelProviderQuotaInvalidator interface {
	InvalidateChannelQuota(ctx context.Context, channelID int) error
}

// ChannelQuotaKeyReconciler lets the provider quota service align quota-owned
// key disables with the per-key quota observation it just collected.
type ChannelQuotaKeyReconciler interface {
	ReconcileQuotaKeys(ctx context.Context, snapshot *ent.Channel, accounts []provider_quota.APIKeyQuota) (*ent.Channel, error)
}

// SetChannelProviderQuotaInvalidator wires the provider quota invalidator once
// at startup. It is optional for tests that construct ChannelService directly.
func (svc *ChannelService) SetChannelProviderQuotaInvalidator(invalidator ChannelProviderQuotaInvalidator) {
	svc.providerQuotaInvalidator = invalidator
}

// SetQuotaKeyReconciler wires the per-key quota reconciler once at startup. It
// is optional for tests that construct ProviderQuotaService directly.
func (svc *ProviderQuotaService) SetQuotaKeyReconciler(reconciler ChannelQuotaKeyReconciler) {
	svc.quotaKeyReconciler = reconciler
}

func (svc *ChannelService) invalidateProviderQuota(ctx context.Context, channelID int) {
	if svc.providerQuotaInvalidator == nil {
		return
	}

	if err := svc.providerQuotaInvalidator.InvalidateChannelQuota(ctx, channelID); err != nil {
		log.Warn(ctx, "failed to invalidate provider quota after channel provider change",
			log.Int("channel_id", channelID),
			log.Cause(err))
	}
}
