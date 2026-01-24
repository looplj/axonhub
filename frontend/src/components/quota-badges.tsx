import { Loader2, RefreshCw, Battery, BatteryLow, BatteryMedium, BatteryFull, BatteryWarning } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useProviderQuotaStatuses, ProviderQuotaChannel, checkProviderQuotas } from '@/features/system/data/quotas';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

const STATUS_COLORS = {
  available: 'bg-green-500 hover:bg-green-600 text-white',
  warning: 'bg-yellow-500 hover:bg-yellow-600 text-white',
  exhausted: 'bg-red-500 hover:bg-red-600 text-white',
  unknown: 'bg-gray-500 hover:bg-gray-600 text-white',
} as const;

const STATUS_LABELS = {
  available: 'Available',
  warning: 'Warning',
  exhausted: 'Exhausted',
  unknown: 'Unknown',
} as const;

type QuotaData = {
  windows?: {
    '5h'?: { utilization?: number; reset?: number; status?: string };
    '7d'?: { utilization?: number; reset?: number; status?: string };
    overage?: { utilization?: number; reset?: number; status?: string };
  };
  representative_claim?: string;
  plan_type?: string;
  rate_limit?: {
    primary_window?: { used_percent?: number; reset_at?: number; limit_window_seconds?: number };
    secondary_window?: { used_percent?: number; reset_at?: number; limit_window_seconds?: number };
  };
};

type BatteryLevel = 'full' | 'medium' | 'low' | 'empty' | 'warning';

function getBatteryIcon(level: BatteryLevel) {
  switch (level) {
    case 'full':
      return BatteryFull;
    case 'medium':
      return BatteryMedium;
    case 'low':
      return BatteryLow;
    case 'warning':
      return BatteryWarning;
    default:
      return Battery;
  }
}

function getBatteryLevel(percentage: number, status: string): BatteryLevel {
  if (status === 'exhausted') return 'warning';
  // percentage is USED percentage, so need to check remaining
  const remaining = 100 - percentage;
  if (remaining < 5) return 'empty';
  if (remaining < 20) return 'low';
  if (remaining < 80) return 'medium';
  return 'full';
}

function getChannelPercentage(channel: ProviderQuotaChannel, quotaData: QuotaData): number {
  let percentage = 0;
  if (channel.type === 'claudecode') {
    const util5h = quotaData.windows?.['5h']?.utilization || 0;
    const util7d = quotaData.windows?.['7d']?.utilization || 0;
    percentage = Math.max(util5h, util7d) * 100;
  } else if (channel.type === 'codex') {
    percentage = quotaData.rate_limit?.primary_window?.used_percent || 0;
  }
  return percentage;
}

function QuotaRow({ channel }: { channel: ProviderQuotaChannel }) {
  const quota = channel.quotaStatus;
  if (!quota) return null;

  const status = quota.status || 'unknown';
  const colorClass = STATUS_COLORS[status as keyof typeof STATUS_COLORS] || STATUS_COLORS.unknown;
  const statusLabel = STATUS_LABELS[status as keyof typeof STATUS_LABELS] || 'Unknown';
  const quotaData = quota.quotaData as QuotaData;

  const percentage = getChannelPercentage(channel, quotaData);
  const batteryLevel = getBatteryLevel(percentage, status);
  const BatteryIcon = getBatteryIcon(batteryLevel);
  const displayPercentage = status === 'unknown' ? '?' : Math.round(percentage);

  const formatWindowDuration = (seconds?: number) => {
    if (!seconds) return 'Unknown';
    const hours = Math.floor(seconds / 3600);
    const days = hours >= 24 ? Math.floor(hours / 24) : 0;
    if (days > 0) return `${days} day${days > 1 ? 's' : ''}`;
    if (hours > 0) return `${hours} hour${hours > 1 ? 's' : ''}`;
    return `${Math.floor(seconds / 60)} min`;
  };

  const formatTimeToReset = (resetAt?: string | null) => {
    if (!resetAt) return 'Unknown';
    const now = Date.now();
    const reset = new Date(resetAt).getTime();
    const diffMs = reset - now;
    if (diffMs < 0) return 'Reset now';
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours > 0) return `${diffHours}h ${diffMins % 60}m`;
    return `${diffMins}m`;
  };

  return (
    <div className="space-y-2 text-sm py-3 first:pt-0 border-b last:border-0 last:pb-0 pb-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <BatteryIcon className={`w-4 h-4 ${status === 'exhausted' ? 'text-red-500' : status === 'warning' ? 'text-yellow-500' : 'text-muted-foreground'}`} />
          <span className="font-medium">{channel.name}</span>
        </div>
        <span className={`text-xs px-2 py-0.5 rounded ${colorClass}`}>{statusLabel}</span>
      </div>

      {channel.type === 'claudecode' && (
        <div className="ml-6 mt-2">
          <div className="space-y-1.5 text-xs">
            <div className="flex justify-between items-center text-muted-foreground">
              <span>Used</span>
              <span className={`font-medium ${batteryLevel === 'warning' || batteryLevel === 'low' ? 'text-red-500' : 'text-foreground'}`}>{displayPercentage}%</span>
            </div>
            <div className="flex justify-between items-center text-muted-foreground">
              <span>5h window</span>
              <span className="font-medium">{Math.round((quotaData.windows?.['5h']?.utilization || 0) * 100)}%</span>
            </div>
            <div className="flex justify-between items-center text-muted-foreground">
              <span>7d window</span>
              <span className="font-medium">{Math.round((quotaData.windows?.['7d']?.utilization || 0) * 100)}%</span>
            </div>
            {quotaData.representative_claim && (
              <div className="flex justify-between items-center text-muted-foreground">
                <span>Limiting bucket</span>
                <span>{quotaData.representative_claim === 'five_hour' ? '5h' : '7d'}</span>
              </div>
            )}
            <div className="flex justify-between items-center text-muted-foreground">
              <span>Reset in</span>
              <span>{formatTimeToReset(quota.nextResetAt)}</span>
            </div>
          </div>
        </div>
      )}

      {channel.type === 'codex' && (
        <div className="ml-6 mt-2">
          <div className="space-y-1.5 text-xs">
            <div className="flex justify-between items-center text-muted-foreground">
              <span>Used</span>
              <span className={`font-medium ${batteryLevel === 'warning' || batteryLevel === 'low' ? 'text-red-500' : 'text-foreground'}`}>{displayPercentage}%</span>
            </div>
            <div className="flex justify-between items-center text-muted-foreground">
              <span>Primary window</span>
              <span className="font-medium">{Math.round(quotaData.rate_limit?.primary_window?.used_percent || 0)}%</span>
            </div>
            <div className="flex justify-between items-center text-muted-foreground">
              <span>Primary duration</span>
              <span>{formatWindowDuration(quotaData.rate_limit?.primary_window?.limit_window_seconds)}</span>
            </div>
            {quotaData.rate_limit?.primary_window?.reset_at && (
              <div className="flex justify-between items-center text-muted-foreground">
                <span>Resets at</span>
                <span>{new Date(quotaData.rate_limit.primary_window.reset_at * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
              </div>
            )}
            {quotaData.plan_type && (
              <div className="flex justify-between items-center text-muted-foreground">
                <span>Plan</span>
                <span>{quotaData.plan_type}</span>
              </div>
            )}
            {quotaData.rate_limit?.secondary_window?.used_percent !== undefined && (
              <>
                <div className="flex justify-between items-center text-muted-foreground">
                  <span>Secondary window</span>
                  <span className="font-medium">{Math.round(quotaData.rate_limit.secondary_window.used_percent)}%</span>
                </div>
                {quotaData.rate_limit?.secondary_window?.limit_window_seconds && (
                  <div className="flex justify-between items-center text-muted-foreground">
                    <span>Secondary duration</span>
                    <span>{formatWindowDuration(quotaData.rate_limit.secondary_window.limit_window_seconds)}</span>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function QuotaBadgeTrigger({ channels }: { channels: ProviderQuotaChannel[] }) {
  // Get worst used percentage (highest used)
  const highestUsed = Math.max(...channels.map(c => {
    const quota = c.quotaStatus;
    if (!quota) return 0;
    const quotaData = quota.quotaData as QuotaData;
    return getChannelPercentage(c, quotaData);
  }));

  const hasExhausted = channels.some(c => c.quotaStatus?.status === 'exhausted');
  const hasWarning = channels.some(c => c.quotaStatus?.status === 'warning');

  let level: BatteryLevel = 'full';
  if (hasExhausted) level = 'warning';
  else if (hasWarning) level = 'low';
  else level = getBatteryLevel(highestUsed, 'available');

  const BatteryIcon = getBatteryIcon(level);
  const isWarning = level === 'warning';
  const textColor = isWarning ? 'text-red-500' : level === 'low' ? 'text-yellow-500' : 'text-muted-foreground';

  return (
    <BatteryIcon className={`w-5 h-5 ${textColor} transition-colors`} />
  );
}

export function QuotaBadges({ isRefreshing, onRefresh }: { isRefreshing: boolean; onRefresh: () => void }) {
  const channels = useProviderQuotaStatuses();

  if (channels.length === 0) return null;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <div className="p-2 hover:bg-muted rounded-md transition-colors relative">
          <QuotaBadgeTrigger channels={channels} />
        </div>
      </PopoverTrigger>
      <PopoverContent className="w-80" align="end">
        <div className="space-y-1">
          <div className="flex items-center justify-between mb-2">
            <div className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Provider Quotas
            </div>
            <button
              onClick={onRefresh}
              disabled={isRefreshing}
              className="text-muted-foreground hover:text-foreground transition-colors"
              aria-label="Refresh quotas"
            >
              {isRefreshing ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <RefreshCw className="w-4 h-4" />
              )}
            </button>
          </div>
          {channels.map((channel: ProviderQuotaChannel) => (
            <QuotaRow key={channel.id} channel={channel} />
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}
