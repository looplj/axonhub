import type { QuotaRoutingMode } from '@/features/system/data/system';
import { parseQuotaLimits } from '@/features/system/data/quotas';
import type { Channel } from '../data/schema';

export type ChannelQuotaRoutingIndicator = 'exhausted' | 'backpressure';

export function getChannelQuotaRoutingIndicator(
  channel: Pick<Channel, 'providerQuotaStatus' | 'settings'>,
  globalDefaultMode?: QuotaRoutingMode
): ChannelQuotaRoutingIndicator | undefined {
  if (channel.providerQuotaStatus?.status === 'exhausted') {
    return 'exhausted';
  }

  const channelMode = channel.settings?.quotaRoutingMode;
  const effectiveMode = channelMode && channelMode !== 'INHERIT' ? channelMode : globalDefaultMode;
  if (effectiveMode === 'BACKPRESSURE' && hasQuotaWindowPressure(channel.providerQuotaStatus?.quotaData)) {
    return 'backpressure';
  }

  return undefined;
}

function hasQuotaWindowPressure(quotaData: unknown): boolean {
  const now = Date.now();
  return parseQuotaLimits(quotaData).some((limit) => {
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
