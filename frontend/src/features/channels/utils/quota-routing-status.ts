import type { QuotaRoutingMode } from '@/features/system/data/system';
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
  if (channel.providerQuotaStatus?.status === 'warning' && effectiveMode === 'BACKPRESSURE') {
    return 'backpressure';
  }

  return undefined;
}
