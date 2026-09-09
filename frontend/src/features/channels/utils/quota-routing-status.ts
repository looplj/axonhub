import type { QuotaRoutingMode } from '@/features/system/data/system';
import { parseQuotaLimits } from '@/features/system/data/quotas';
import type { Channel } from '../data/schema';

export type ChannelQuotaRoutingIndicator = 'exhausted' | 'backpressure';
type QuotaLimit = { window: string; status: string; usageRatio: number; periodStart?: string | null; nextResetAt?: string | null };

export function getChannelQuotaRoutingIndicator(
  channel: Pick<Channel, 'providerQuotaStatus' | 'settings'> & { quotaRoutingMode?: QuotaRoutingMode | null },
  globalDefaultMode?: QuotaRoutingMode,
  effectiveModeOverride?: QuotaRoutingMode | null,
  limits?: readonly QuotaLimit[],
  statusOverride?: string
): ChannelQuotaRoutingIndicator | undefined {
  const channelMode = channel.settings?.quotaRoutingMode ?? channel.quotaRoutingMode;
  const effectiveMode = effectiveModeOverride ?? (channelMode && channelMode !== 'INHERIT' ? channelMode : globalDefaultMode);
  if (effectiveMode === 'IGNORE_QUOTA') return undefined;

  if ((statusOverride ?? channel.providerQuotaStatus?.status) === 'exhausted') return 'exhausted';

  if (effectiveMode === 'BACKPRESSURE' && hasQuotaWindowPressure(channel.providerQuotaStatus?.quotaData, limits)) {
    return 'backpressure';
  }

  return undefined;
}

function hasQuotaWindowPressure(quotaData: unknown, providedLimits?: readonly QuotaLimit[]): boolean {
  const now = Date.now();
  return (providedLimits ?? parseQuotaLimits(quotaData)).some((limit) => {
    if (limit.window === 'pay_as_you_go' || limit.window === 'credits' || limit.status === 'exhausted' || !limit.periodStart || !limit.nextResetAt) {
      return false;
    }

    const periodStart = Date.parse(limit.periodStart);
    const nextResetAt = Date.parse(limit.nextResetAt);
    if (!Number.isFinite(periodStart) || !Number.isFinite(nextResetAt) || periodStart >= now || nextResetAt <= now || nextResetAt <= periodStart) {
      return false;
    }

    const elapsedRatio = (now - periodStart) / (nextResetAt - periodStart);
    return limit.usageRatio > elapsedRatio;
  });
}
