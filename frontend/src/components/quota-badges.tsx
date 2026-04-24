import { Loader2, RefreshCw, Battery, BatteryLow, BatteryMedium, BatteryFull, BatteryWarning } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useProviderQuotaStatuses, ProviderQuotaChannel } from '@/features/system/data/quotas';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';


const STATUS_LABELS = {
  available: 'quota.status.available',
  warning: 'quota.status.warning',
  exhausted: 'quota.status.exhausted',
  unknown: 'quota.status.unknown',
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
    primary_window?: {
      used_percent?: number;
      reset_at?: number;
      reset_after_seconds?: number;
      limit_window_seconds?: number;
    };
    secondary_window?: {
      used_percent?: number;
      reset_at?: number;
      reset_after_seconds?: number;
      limit_window_seconds?: number;
    };
  };
  error?: string;
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


function ProgressBar({ percentage, type = 'usage', durationPercentage }: { percentage: number; type?: 'usage' | 'duration'; durationPercentage?: number }) {
  const clamped = Math.min(Math.max(percentage || 0, 0), 100);
  
  let bgStyle = {};
  if (type === 'duration') {
    bgStyle = { backgroundColor: '#71717a' }; // zinc-500
  } else {
    let u = clamped / 100;
    let severity = u;
    if (durationPercentage !== undefined && durationPercentage > 0) {
      let d = Math.max(durationPercentage / 100, 0.01);
      severity = u * (u / d);
    }
    severity = Math.min(1, Math.max(0, severity));
    
    // Tailwind 500 colors approximation for a modern, theme-friendly gradient:
    // Green (142, 71%, 45%), Yellow (45, 93%, 47%), Red (0, 84%, 60%)
    let h, s, l;
    if (severity < 0.5) {
      const n = severity * 2; // 0 to 1
      h = 142 - n * (142 - 45);
      s = 71 + n * (93 - 71);
      l = 45 + n * (47 - 45);
    } else {
      const n = (severity - 0.5) * 2; // 0 to 1
      h = 45 - n * 45;
      s = 93 - n * (93 - 84);
      l = 47 + n * (60 - 47);
    }
    bgStyle = { backgroundColor: `hsl(${Math.round(h)}, ${Math.round(s)}%, ${Math.round(l)}%)` };
  }
  
  return (
    <div className="h-1.5 w-full bg-muted/60 rounded-full overflow-hidden">
      <div 
        className="h-full transition-all duration-500" 
        style={{ width: `${clamped}%`, ...bgStyle }} 
      />
    </div>
  );
}

function QuotaRow({ channel }: { channel: ProviderQuotaChannel }) {
  const { t } = useTranslation();
  const quota = channel.quotaStatus;
  if (!quota) return null;

  const status = quota.status || 'unknown';
  const statusLabel = t(STATUS_LABELS[status as keyof typeof STATUS_LABELS]);
  const quotaData = quota.quotaData as QuotaData;

  const percentage = getChannelPercentage(channel, quotaData);
  const batteryLevel = getBatteryLevel(percentage, status);
  const BatteryIcon = getBatteryIcon(batteryLevel);

  const formatWindowDuration = (seconds?: number) => {
    if (!seconds) return '';
    const hours = Math.floor(seconds / 3600);
    const days = hours >= 24 ? Math.floor(hours / 24) : 0;
    if (days > 0) return `${days}${t(days > 1 ? 'quota.label.days' : 'quota.label.day', { defaultValue: days > 1 ? ' days' : ' day' })}`;
    if (hours > 0) return `${hours}${t(hours > 1 ? 'quota.label.hours' : 'quota.label.hour', { defaultValue: hours > 1 ? ' hours' : ' hour' })}`;
    return `${Math.floor(seconds / 60)}${t('quota.label.mins', { defaultValue: ' mins' })}`;
  };

  const calcDurationPercent = (limit?: number, resetAfter?: number) => {
    if (!limit || resetAfter === undefined) return 0;
    const elapsed = limit - resetAfter;
    return Math.max(0, Math.min(100, (elapsed / limit) * 100));
  };

  const getClaudeDurationPercent = (windowKey: string, resetTs?: number) => {
    if (!resetTs) return undefined;
    let limit = 0;
    if (windowKey === '5h') limit = 5 * 3600;
    else if (windowKey === '7d') limit = 7 * 24 * 3600;
    else return undefined;
    
    const now = Date.now() / 1000;
    const resetAfter = resetTs - now;
    return calcDurationPercent(limit, resetAfter);
  };

  const formatTimeToReset = (resetAtOrSeconds?: string | number | null) => {
    if (!resetAtOrSeconds) return '';
    
    let resetTimeMs: number;
    if (typeof resetAtOrSeconds === 'number') {
      resetTimeMs = Date.now() + resetAtOrSeconds * 1000;
    } else {
      resetTimeMs = new Date(resetAtOrSeconds).getTime();
    }

    const now = Date.now();
    const diffMs = resetTimeMs - now;
    if (diffMs < 0) return t('quota.label.reset_now', { defaultValue: 'Reset now' });
    
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    const d = t('quota.label.d', { defaultValue: 'd' });
    const h = t('quota.label.h', { defaultValue: 'h' });
    const m = t('quota.label.m', { defaultValue: 'm' });

    if (diffDays > 0) return `${diffDays}${d} ${diffHours % 24}${h}`;
    if (diffHours > 0) return `${diffHours}${h} ${diffMins % 60}${m}`;
    return `${diffMins}${m}`;
  };

  const formatDate = (timestamp?: number) => {
    if (!timestamp) return '';
    const date = new Date(timestamp * 1000);
    const now = new Date();
    
    if (date.toDateString() === now.toDateString()) {
      return `${t('quota.label.today', { defaultValue: 'Today' })}, ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })}`;
    }
    
    if (date.getFullYear() === now.getFullYear()) {
      return format(date, 'MM-dd HH:mm');
    }
    
    return format(date, 'yyyy-MM-dd HH:mm');
  };

  return (
    <div className="space-y-3 py-3 first:pt-1 border-b last:border-0 last:pb-1">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <BatteryIcon className={`w-4 h-4 ${status === 'exhausted' ? 'text-red-500' : status === 'warning' ? 'text-yellow-500' : 'text-muted-foreground'}`} />
          <span className="font-medium text-foreground">{channel.name}</span>
          {quotaData.plan_type && (
            <Badge variant="outline" className="px-1.5 py-0 h-4 text-[10px] uppercase tracking-wider text-muted-foreground font-semibold">
              {quotaData.plan_type}
            </Badge>
          )}
        </div>
        <Badge 
          variant={status === 'available' ? 'outline' : status === 'warning' ? 'secondary' : status === 'exhausted' ? 'destructive' : 'outline'} 
          className={status === 'available' ? 'bg-green-500/10 text-green-500 border-green-500/20 hover:bg-green-500/20' : ''}
        >
          {statusLabel}
        </Badge>
      </div>

      {quotaData.error && (
        <div className="ml-6 text-xs text-red-500 break-words bg-red-500/10 p-2 rounded">
          <span className="font-medium">{t('quota.label.error')}:</span> {quotaData.error}
        </div>
      )}

      {channel.type === 'claudecode' && (
        <div className="mt-4 space-y-4">
          {quotaData.windows?.['5h'] && (
            <div className="space-y-2.5">
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">{t('quota.window.5h')}</span>
                  <span className="font-medium text-foreground">{Math.round((quotaData.windows['5h'].utilization || 0) * 100)}%</span>
                </div>
                <ProgressBar 
                  percentage={(quotaData.windows['5h'].utilization || 0) * 100} 
                  durationPercentage={getClaudeDurationPercent('5h', quotaData.windows['5h'].reset)}
                />
              </div>
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">
                    5h duration
                  </span>
                  <span className="font-medium text-foreground">{Math.round(getClaudeDurationPercent('5h', quotaData.windows['5h'].reset) || 0)}%</span>
                </div>
                <ProgressBar 
                  type="duration"
                  percentage={getClaudeDurationPercent('5h', quotaData.windows['5h'].reset) || 0} 
                />
              </div>
            </div>
          )}
          {quotaData.windows?.['7d'] && (
            <div className="space-y-2.5 pt-3 border-t border-dashed border-border/60">
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">{t('quota.window.7d')}</span>
                  <span className="font-medium text-foreground">{Math.round((quotaData.windows['7d'].utilization || 0) * 100)}%</span>
                </div>
                <ProgressBar 
                  percentage={(quotaData.windows['7d'].utilization || 0) * 100} 
                  durationPercentage={getClaudeDurationPercent('7d', quotaData.windows['7d'].reset)}
                />
              </div>
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">
                    7d duration
                  </span>
                  <span className="font-medium text-foreground">{Math.round(getClaudeDurationPercent('7d', quotaData.windows['7d'].reset) || 0)}%</span>
                </div>
                <ProgressBar 
                  type="duration"
                  percentage={getClaudeDurationPercent('7d', quotaData.windows['7d'].reset) || 0} 
                />
              </div>
            </div>
          )}
          {quotaData.windows?.['overage'] && (
            <div className="space-y-2.5 pt-3 border-t border-dashed border-border/60">
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">Overage window</span>
                  <span className="font-medium text-foreground">{Math.round((quotaData.windows['overage'].utilization || 0) * 100)}%</span>
                </div>
                <ProgressBar 
                  percentage={(quotaData.windows['overage'].utilization || 0) * 100} 
                />
              </div>
            </div>
          )}
          
          {(quota.nextResetAt || quotaData.representative_claim) && (
             <div className="flex justify-between items-center text-[11px] text-muted-foreground pt-1">
                <span>{quotaData.representative_claim === 'five_hour' ? t('quota.label.5h_limiting', { defaultValue: '5h limiting' }) : quotaData.representative_claim === 'seven_day' ? t('quota.label.7d_limiting', { defaultValue: '7d limiting' }) : ''}</span>
                {quota.nextResetAt && (
                  <span>{t('quota.label.resets_in', { defaultValue: 'Resets in' })} {formatTimeToReset(quota.nextResetAt)} ({formatDate(new Date(quota.nextResetAt).getTime() / 1000)})</span>
                )}
             </div>
          )}
        </div>
      )}

      {channel.type === 'codex' && (
        <div className="mt-4 space-y-4">
          {quotaData.rate_limit?.primary_window && (
            <div className="space-y-2.5">
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">{t('quota.label.primary_window')}</span>
                  <span className="font-medium text-foreground">{Math.round(quotaData.rate_limit.primary_window.used_percent || 0)}%</span>
                </div>
                <ProgressBar 
                  percentage={quotaData.rate_limit.primary_window.used_percent || 0} 
                  durationPercentage={quotaData.rate_limit.primary_window.limit_window_seconds ? calcDurationPercent(quotaData.rate_limit.primary_window.limit_window_seconds, quotaData.rate_limit.primary_window.reset_after_seconds) : undefined}
                />
              </div>

              {quotaData.rate_limit.primary_window.limit_window_seconds ? (
                <div className="space-y-1">
                  <div className="flex justify-between items-center text-xs">
                    <span className="font-medium text-muted-foreground">
                      {t('quota.label.primary_duration')} ({formatWindowDuration(quotaData.rate_limit.primary_window.limit_window_seconds)})
                    </span>
                    <span className="font-medium text-foreground">{Math.round(calcDurationPercent(quotaData.rate_limit.primary_window.limit_window_seconds, quotaData.rate_limit.primary_window.reset_after_seconds))}%</span>
                  </div>
                  <ProgressBar 
                    type="duration"
                    percentage={calcDurationPercent(quotaData.rate_limit.primary_window.limit_window_seconds, quotaData.rate_limit.primary_window.reset_after_seconds)} 
                  />
                </div>
              ) : null}

              {quotaData.rate_limit.primary_window.reset_at && (
                <div className="text-[11px] text-muted-foreground text-right pt-0.5">
                  {t('quota.label.resets_in', { defaultValue: 'Resets in' })} {formatTimeToReset(quotaData.rate_limit.primary_window.reset_after_seconds)} ({formatDate(quotaData.rate_limit.primary_window.reset_at)})
                </div>
              )}
            </div>
          )}

          {quotaData.rate_limit?.secondary_window?.used_percent !== undefined && (
            <div className="space-y-2.5 pt-3 mt-3 border-t border-dashed border-border/60">
              <div className="space-y-1">
                <div className="flex justify-between items-center text-xs">
                  <span className="font-medium text-muted-foreground">{t('quota.label.secondary_window')}</span>
                  <span className="font-medium text-foreground">{Math.round(quotaData.rate_limit.secondary_window.used_percent)}%</span>
                </div>
                <ProgressBar 
                  percentage={quotaData.rate_limit.secondary_window.used_percent} 
                  durationPercentage={quotaData.rate_limit.secondary_window.limit_window_seconds ? calcDurationPercent(quotaData.rate_limit.secondary_window.limit_window_seconds, quotaData.rate_limit.secondary_window.reset_after_seconds) : undefined}
                />
              </div>

              {quotaData.rate_limit.secondary_window.limit_window_seconds ? (
                <div className="space-y-1">
                  <div className="flex justify-between items-center text-xs">
                    <span className="font-medium text-muted-foreground">
                      {t('quota.label.secondary_duration')} ({formatWindowDuration(quotaData.rate_limit.secondary_window.limit_window_seconds)})
                    </span>
                    <span className="font-medium text-foreground">{Math.round(calcDurationPercent(quotaData.rate_limit.secondary_window.limit_window_seconds, quotaData.rate_limit.secondary_window.reset_after_seconds))}%</span>
                  </div>
                  <ProgressBar 
                    type="duration"
                    percentage={calcDurationPercent(quotaData.rate_limit.secondary_window.limit_window_seconds, quotaData.rate_limit.secondary_window.reset_after_seconds)} 
                  />
                </div>
              ) : null}

              {quotaData.rate_limit.secondary_window.reset_at && (
                <div className="text-[11px] text-muted-foreground text-right pt-0.5">
                  {t('quota.label.resets_in', { defaultValue: 'Resets in' })} {formatTimeToReset(quotaData.rate_limit.secondary_window.reset_after_seconds)} ({formatDate(quotaData.rate_limit.secondary_window.reset_at)})
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function QuotaBadgeTrigger({ channels }: { channels: ProviderQuotaChannel[] }) {
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
  const { t } = useTranslation();
  const channels = useProviderQuotaStatuses();

  if (channels.length === 0) return null;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button type="button" className="p-2 hover:bg-muted rounded-md transition-colors relative">
          <QuotaBadgeTrigger channels={channels} />
        </button>
      </PopoverTrigger>
      <PopoverContent className={channels.length > 4 ? "w-[640px]" : "w-80"} align="end">
        <div className="space-y-1">
          <div className="flex items-center justify-between mb-2">
            <div className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              {t('system.providerQuota.title')}
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
          <div className={`max-h-[60vh] overflow-y-auto ${channels.length > 4 ? 'grid grid-cols-2 gap-x-4' : ''}`}>
            {channels.map((channel: ProviderQuotaChannel) => (
              <QuotaRow key={channel.id} channel={channel} />
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
