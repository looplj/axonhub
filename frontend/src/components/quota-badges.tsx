import { format } from 'date-fns';
import { Loader2, RefreshCw, Battery, BatteryLow, BatteryMedium, BatteryFull, BatteryWarning } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  useProviderQuotaStatuses,
  ProviderQuotaChannel,
  ProviderNanoGPTQuotaData,
  NanoGPTQuotaWindow,
  ProviderWaferQuotaData,
  ProviderSyntheticQuotaData,
  ProviderNeuralWattQuotaData,
} from '@/features/system/data/quotas';
import { useQuotaEnforcementSettings, type QuotaEnforcementMode } from '@/features/system/data/system';

const syntheticWeeklyRegenTickPct = 0.02;

const BADGE_COLOR_CLASSES: Record<string, string> = {
  green: 'bg-green-500/10 text-green-500 border-green-500/20 hover:bg-green-500/20',
  red: 'bg-red-500/10 text-red-500 border-red-500/20 hover:bg-red-500/20',
  amber: 'bg-amber-500/10 text-amber-500 border-amber-500/20 hover:bg-amber-500/20',
};

const STATUS_LABELS = {
  available: 'quota.status.available',
  warning: 'quota.status.warning',
  exhausted: 'quota.status.exhausted',
  unknown: 'quota.status.unknown',
} as const;

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

function getChannelPercentage(channel: ProviderQuotaChannel): number {
  let percentage = 0;
  if (!channel.quotaStatus) return 0;

  if (channel.type === 'claudecode') {
    const qd = channel.quotaStatus.quotaData;
    const util5h = qd.windows?.['5h']?.utilization || 0;
    const util7d = qd.windows?.['7d']?.utilization || 0;
    percentage = Math.max(util5h, util7d) * 100;
  } else if (channel.type === 'codex') {
    const qd = channel.quotaStatus.quotaData;
    percentage = qd.rate_limit?.primary_window?.used_percent || 0;
  } else if (channel.type === 'github_copilot') {
    const qd = channel.quotaStatus.quotaData;
    let lowestRemaining = 100;
    const limitedQuotas = qd.limited_user_quotas;
    const totalQuotas = qd.total_quotas;

    if (limitedQuotas) {
      Object.entries(limitedQuotas).forEach(([key, remaining]) => {
        if (typeof remaining === 'number') {
          const total = totalQuotas?.[key] ?? remaining;
          if (total > 0) {
            lowestRemaining = Math.min(lowestRemaining, (remaining / total) * 100);
          }
        }
      });
    }

    if (qd.quota_snapshots) {
      Object.values(qd.quota_snapshots).forEach((snapshot) => {
        if (snapshot && !snapshot.unlimited && typeof snapshot.percent_remaining === 'number') {
          lowestRemaining = Math.min(lowestRemaining, snapshot.percent_remaining);
        }
      });
    }

    percentage = 100 - lowestRemaining;
  } else if (channel.type === 'nanogpt' || channel.type === 'nanogpt_responses') {
    const qd = channel.quotaStatus?.quotaData;
    if (!qd) return 0;
    let maxPercent = 0;
    if (qd.windows?.weeklyInputTokens) maxPercent = Math.max(maxPercent, (qd.windows.weeklyInputTokens.percentUsed ?? 0) * 100);
    if (qd.windows?.dailyInputTokens) maxPercent = Math.max(maxPercent, (qd.windows.dailyInputTokens.percentUsed ?? 0) * 100);
    if (qd.windows?.dailyImages) maxPercent = Math.max(maxPercent, (qd.windows.dailyImages.percentUsed ?? 0) * 100);
    percentage = maxPercent;
  } else if (channel.type === 'openai' && channel.providerType === 'wafer') {
    const qd = channel.quotaStatus?.quotaData as ProviderWaferQuotaData | undefined;
    percentage = qd?.current_period_used_percent ?? 0;
  } else if (channel.type === 'openai' && channel.providerType === 'synthetic') {
    const qd = channel.quotaStatus?.quotaData as ProviderSyntheticQuotaData | undefined;
    const weeklyPct = qd?.weeklyTokenLimit?.percentRemaining ?? 100;
    percentage = 100 - weeklyPct;
  } else if (channel.type === 'openai' && channel.providerType === 'neuralwatt') {
    const qd = channel.quotaStatus?.quotaData as ProviderNeuralWattQuotaData | undefined;
    const kwhIncluded = qd?.subscription?.kwh_included ?? 0;
    const kwhUsed = qd?.subscription?.kwh_used ?? 0;
    if (kwhIncluded > 0) {
      percentage = (kwhUsed / kwhIncluded) * 100;
    }
  }
  return percentage;
}

function ProgressBar({
  percentage,
  type = 'usage',
  durationPercentage,
}: {
  percentage: number;
  type?: 'usage' | 'duration';
  durationPercentage?: number;
}) {
  const clamped = Math.min(Math.max(percentage || 0, 0), 100);

  let bgStyle = {};
  if (type === 'duration') {
    bgStyle = { backgroundColor: '#71717a' }; // zinc-500
  } else {
    const u = clamped / 100;
    let severity = u;
    if (durationPercentage !== undefined && durationPercentage > 0) {
      const d = Math.max(durationPercentage / 100, 0.01);
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
    <div className='bg-muted/60 h-1.5 w-full overflow-hidden rounded-full'>
      <div className='h-full transition-all duration-500' style={{ width: `${clamped}%`, ...bgStyle }} />
    </div>
  );
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return `${n}`;
}

function QuotaRow({ channel, enforcementMode }: { channel: ProviderQuotaChannel; enforcementMode?: QuotaEnforcementMode | null }) {
  const { t } = useTranslation();
  const quota = channel.quotaStatus;
  if (!quota) return null;

  const status = quota.status;
  const statusLabel = t(STATUS_LABELS[status]);

  const enforcementEffect =
    enforcementMode && (status === 'exhausted' || (status === 'warning' && enforcementMode === 'DE_PRIORITIZE'))
      ? enforcementMode === 'EXHAUSTED_ONLY'
        ? ('blocked' as const)
        : ('deprioritized' as const)
      : null;

  const percentage = getChannelPercentage(channel);
  const batteryLevel = getBatteryLevel(percentage, status);
  const BatteryIcon = getBatteryIcon(batteryLevel);

  const formatWindowDuration = (seconds?: number) => {
    if (!seconds) return '';
    const hours = Math.floor(seconds / 3600);
    const days = hours >= 24 ? Math.floor(hours / 24) : 0;
    if (days > 0) return `${days}${t(days > 1 ? 'quota.label.days' : 'quota.label.day')}`;
    if (hours > 0) return `${hours}${t(hours > 1 ? 'quota.label.hours' : 'quota.label.hour')}`;
    return `${Math.floor(seconds / 60)}${t('quota.label.mins')}`;
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

  const formatTimeToReset = (resetAtOrSeconds?: string | number | null, usedPercent?: number, regenerates?: boolean | number) => {
    if (!resetAtOrSeconds) return '';

    let resetTimeMs: number;
    if (typeof resetAtOrSeconds === 'number') {
      resetTimeMs = Date.now() + resetAtOrSeconds * 1000;
    } else {
      resetTimeMs = new Date(resetAtOrSeconds).getTime();
    }

    if (usedPercent === 0) return t('quota.label.no_usage_yet');

    const now = Date.now();
    const diffMs = resetTimeMs - now;
    const isRegen = regenerates != null && regenerates !== false;
    const regenPct = typeof regenerates === 'number' ? Math.round(regenerates * 100) : null;
    if (diffMs < 0) return isRegen ? t('quota.label.regenerating_now') : t('quota.label.reset_now');

    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    const d = t('quota.label.d');
    const h = t('quota.label.h');
    const m = t('quota.label.m');

    let timeStr: string;
    if (diffDays > 0) timeStr = `${diffDays}${d} ${diffHours % 24}${h}`;
    else if (diffHours > 0) timeStr = `${diffHours}${h} ${diffMins % 60}${m}`;
    else timeStr = `${diffMins}${m}`;

    if (isRegen && regenPct != null) {
      return t('quota.label.regenerates_pct_in_time', { percent: regenPct, time: timeStr });
    }
    return isRegen ? t('quota.label.regenerates_in_time', { time: timeStr }) : t('quota.label.resets_in_time', { time: timeStr });
  };

  const formatDate = (timestamp?: number) => {
    if (!timestamp) return '';
    const date = new Date(timestamp * 1000);
    const now = new Date();

    if (date.toDateString() === now.toDateString()) {
      return `${t('quota.label.today')}, ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })}`;
    }

    if (date.getFullYear() === now.getFullYear()) {
      return format(date, 'MM-dd HH:mm');
    }

    return format(date, 'yyyy-MM-dd HH:mm');
  };
  const quotaData = quota.quotaData;
  return (
    <div className='space-y-3 border-b py-3 first:pt-1 last:border-0 last:pb-1'>
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-2'>
          <BatteryIcon
            className={`h-4 w-4 ${status === 'exhausted' ? 'text-red-500' : status === 'warning' ? 'text-yellow-500' : 'text-muted-foreground'}`}
          />
          <span className='text-foreground font-medium'>{channel.name}</span>
        </div>
        <div className='flex items-center gap-1.5'>
          <Badge
            variant={
              status === 'available' ? 'outline' : status === 'warning' ? 'secondary' : status === 'exhausted' ? 'destructive' : 'outline'
            }
            className={status === 'available' ? BADGE_COLOR_CLASSES.green : ''}
          >
            {statusLabel}
          </Badge>
          {enforcementEffect && (
            <Badge variant='outline' className={BADGE_COLOR_CLASSES[enforcementEffect === 'blocked' ? 'red' : 'amber']}>
              {t(`quota.status.${enforcementEffect}`)}
            </Badge>
          )}
        </div>
      </div>

      {quotaData.error && (
        <div className='ml-6 rounded bg-red-500/10 p-2 text-xs break-words text-red-500'>
          <span className='font-medium'>{t('quota.label.error')}:</span> {quotaData.error}
        </div>
      )}

      {channel.type === 'claudecode' && (
        <div className='mt-4 space-y-4'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData;
            if (!qd) return null;
            return (
              <>
                {qd.windows?.['5h'] && (
                  <div className='space-y-2.5'>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.window.5h')}</span>
                        <span className='text-foreground font-medium'>{Math.round((qd.windows['5h'].utilization || 0) * 100)}%</span>
                      </div>
                      <ProgressBar
                        percentage={(qd.windows['5h'].utilization || 0) * 100}
                        durationPercentage={getClaudeDurationPercent('5h', qd.windows['5h'].reset)}
                      />
                    </div>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.label.5h_duration')}</span>
                        <span className='text-foreground font-medium'>
                          {Math.round(getClaudeDurationPercent('5h', qd.windows['5h'].reset) || 0)}%
                        </span>
                      </div>
                      <ProgressBar type='duration' percentage={getClaudeDurationPercent('5h', qd.windows['5h'].reset) || 0} />
                    </div>
                  </div>
                )}
                {qd.windows?.['7d'] && (
                  <div className='border-border/60 space-y-2.5 border-t border-dashed pt-3'>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.window.7d')}</span>
                        <span className='text-foreground font-medium'>{Math.round((qd.windows['7d'].utilization || 0) * 100)}%</span>
                      </div>
                      <ProgressBar
                        percentage={(qd.windows['7d'].utilization || 0) * 100}
                        durationPercentage={getClaudeDurationPercent('7d', qd.windows['7d'].reset)}
                      />
                    </div>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.label.7d_duration')}</span>
                        <span className='text-foreground font-medium'>
                          {Math.round(getClaudeDurationPercent('7d', qd.windows['7d'].reset) || 0)}%
                        </span>
                      </div>
                      <ProgressBar type='duration' percentage={getClaudeDurationPercent('7d', qd.windows['7d'].reset) || 0} />
                    </div>
                  </div>
                )}
                {qd.windows?.['overage'] && (
                  <div className='border-border/60 space-y-2.5 border-t border-dashed pt-3'>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.label.overage_window')}</span>
                        <span className='text-foreground font-medium'>{Math.round((qd.windows['overage'].utilization || 0) * 100)}%</span>
                      </div>
                      <ProgressBar percentage={(qd.windows['overage'].utilization || 0) * 100} />
                    </div>
                  </div>
                )}

                {(quota.nextResetAt || qd.representative_claim) && (
                  <div className='text-muted-foreground flex items-center justify-between pt-1 text-[11px]'>
                    <span>
                      {qd.representative_claim === 'five_hour'
                        ? t('quota.label.5h_limiting')
                        : qd.representative_claim === 'seven_day'
                          ? t('quota.label.7d_limiting')
                          : ''}
                    </span>
                    {quota.nextResetAt && (
                      <span>
                        {formatTimeToReset(quota.nextResetAt)} ({formatDate(new Date(quota.nextResetAt).getTime() / 1000)})
                      </span>
                    )}
                  </div>
                )}
              </>
            );
          })()}
        </div>
      )}

      {channel.type === 'github_copilot' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData;
            if (!qd) return null;
            const items: React.ReactNode[] = [];
            const limited = qd.limited_user_quotas;
            const total = qd.total_quotas;

            if (limited) {
              Object.entries(limited).forEach(([key, rem]) => {
                if (typeof rem === 'number') {
                  const tot = total?.[key] ?? rem;
                  const displayRem = rem / 10;
                  const displayTot = tot / 10;
                  const usedPct = tot > 0 ? (1 - rem / tot) * 100 : 0;
                  const labelKey =
                    key === 'completions' ? 'quota.label.inline_suggestions' : key === 'chat' ? 'quota.label.chat_messages' : '';
                  const label = labelKey ? t(labelKey) : key.replace(/_/g, ' ');

                  items.push(
                    <div key={key} className='border-border/60 space-y-2.5 border-t border-dashed pt-3 first:border-0 first:pt-0'>
                      <div className='space-y-1'>
                        <div className='flex items-center justify-between text-xs'>
                          <span className='text-muted-foreground font-medium'>
                            {label}{' '}
                            <span className='font-normal opacity-70'>
                              ({Math.round(displayRem)}/{Math.round(displayTot)})
                            </span>
                          </span>
                          <span className='text-foreground font-medium'>
                            {t('quota.label.percent_used', { percent: Math.round(usedPct) })}
                          </span>
                        </div>
                        <ProgressBar percentage={usedPct} />
                      </div>
                    </div>
                  );
                }
              });
            }

            if (qd.quota_snapshots) {
              Object.entries(qd.quota_snapshots).forEach(([key, snapshot]) => {
                if (snapshot) {
                  const usedPct = snapshot.unlimited ? 0 : 100 - (snapshot.percent_remaining || 0);
                  const labelKey =
                    key === 'premium_interactions'
                      ? 'quota.label.premium_interactions'
                      : key === 'premium_models'
                        ? 'quota.label.premium_models'
                        : key === 'completions'
                          ? 'quota.label.inline_suggestions'
                          : key === 'chat'
                            ? 'quota.label.chat_messages'
                            : '';
                  const label = labelKey ? t(labelKey) : key.replace(/_/g, ' ');

                  const displayRem = snapshot.quota_remaining || snapshot.remaining || 0;
                  const displayTot = snapshot.entitlement || 0;

                  items.push(
                    <div key={key} className='border-border/60 space-y-2.5 border-t border-dashed pt-3 first:border-0 first:pt-0'>
                      <div className='space-y-1'>
                        <div className='flex items-center justify-between text-xs'>
                          <span className='text-muted-foreground font-medium'>
                            {label}{' '}
                            {!snapshot.unlimited && (
                              <span className='font-normal opacity-70'>
                                ({Math.round(displayRem)}/{Math.round(displayTot)})
                              </span>
                            )}
                          </span>
                          <span className='text-foreground font-medium'>
                            {snapshot.unlimited
                              ? t('quota.label.unlimited')
                              : t('quota.label.percent_used', { percent: Math.round(usedPct) })}
                          </span>
                        </div>
                        {!snapshot.unlimited && <ProgressBar percentage={usedPct} />}
                      </div>
                    </div>
                  );
                }
              });
            }

            return items;
          })()}

          {quota.nextResetAt && (
            <div className='text-muted-foreground pt-1 text-right text-[11px]'>
              {formatTimeToReset(quota.nextResetAt)} ({formatDate(new Date(quota.nextResetAt).getTime() / 1000)})
            </div>
          )}
        </div>
      )}

      {channel.type === 'codex' && (
        <div className='mt-4 space-y-4'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData;
            if (!qd) return null;
            return (
              <>
                {qd.rate_limit?.primary_window && (
                  <div className='space-y-2.5'>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.label.primary_window')}</span>
                        <span className='text-foreground font-medium'>{Math.round(qd.rate_limit.primary_window.used_percent || 0)}%</span>
                      </div>
                      <ProgressBar
                        percentage={qd.rate_limit.primary_window.used_percent || 0}
                        durationPercentage={
                          qd.rate_limit.primary_window.limit_window_seconds
                            ? calcDurationPercent(
                                qd.rate_limit.primary_window.limit_window_seconds,
                                qd.rate_limit.primary_window.reset_after_seconds
                              )
                            : undefined
                        }
                      />
                    </div>

                    {qd.rate_limit.primary_window.limit_window_seconds ? (
                      <div className='space-y-1'>
                        <div className='flex items-center justify-between text-xs'>
                          <span className='text-muted-foreground font-medium'>
                            {t('quota.label.primary_duration')} ({formatWindowDuration(qd.rate_limit.primary_window.limit_window_seconds)})
                          </span>
                          <span className='text-foreground font-medium'>
                            {Math.round(
                              calcDurationPercent(
                                qd.rate_limit.primary_window.limit_window_seconds,
                                qd.rate_limit.primary_window.reset_after_seconds
                              )
                            )}
                            %
                          </span>
                        </div>
                        <ProgressBar
                          type='duration'
                          percentage={calcDurationPercent(
                            qd.rate_limit.primary_window.limit_window_seconds,
                            qd.rate_limit.primary_window.reset_after_seconds
                          )}
                        />
                      </div>
                    ) : null}

                    {qd.rate_limit.primary_window.reset_at && (
                      <div className='text-muted-foreground pt-0.5 text-right text-[11px]'>
                        {formatTimeToReset(qd.rate_limit.primary_window.reset_after_seconds)} (
                        {formatDate(qd.rate_limit.primary_window.reset_at)})
                      </div>
                    )}
                  </div>
                )}

                {qd.rate_limit?.secondary_window?.used_percent !== undefined && (
                  <div className='border-border/60 mt-3 space-y-2.5 border-t border-dashed pt-3'>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>{t('quota.label.secondary_window')}</span>
                        <span className='text-foreground font-medium'>{Math.round(qd.rate_limit.secondary_window.used_percent)}%</span>
                      </div>
                      <ProgressBar
                        percentage={qd.rate_limit.secondary_window.used_percent}
                        durationPercentage={
                          qd.rate_limit.secondary_window.limit_window_seconds
                            ? calcDurationPercent(
                                qd.rate_limit.secondary_window.limit_window_seconds,
                                qd.rate_limit.secondary_window.reset_after_seconds
                              )
                            : undefined
                        }
                      />
                    </div>

                    {qd.rate_limit.secondary_window.limit_window_seconds ? (
                      <div className='space-y-1'>
                        <div className='flex items-center justify-between text-xs'>
                          <span className='text-muted-foreground font-medium'>
                            {t('quota.label.secondary_duration')} (
                            {formatWindowDuration(qd.rate_limit.secondary_window.limit_window_seconds)})
                          </span>
                          <span className='text-foreground font-medium'>
                            {Math.round(
                              calcDurationPercent(
                                qd.rate_limit.secondary_window.limit_window_seconds,
                                qd.rate_limit.secondary_window.reset_after_seconds
                              )
                            )}
                            %
                          </span>
                        </div>
                        <ProgressBar
                          type='duration'
                          percentage={calcDurationPercent(
                            qd.rate_limit.secondary_window.limit_window_seconds,
                            qd.rate_limit.secondary_window.reset_after_seconds
                          )}
                        />
                      </div>
                    ) : null}

                    {qd.rate_limit.secondary_window.reset_at && (
                      <div className='text-muted-foreground pt-0.5 text-right text-[11px]'>
                        {formatTimeToReset(qd.rate_limit.secondary_window.reset_after_seconds)} (
                        {formatDate(qd.rate_limit.secondary_window.reset_at)})
                      </div>
                    )}
                  </div>
                )}
              </>
            );
          })()}
        </div>
      )}

      {(channel.type === 'nanogpt' || channel.type === 'nanogpt_responses') && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData as ProviderNanoGPTQuotaData | undefined;
            if (!qd) return null;
            const items: React.ReactNode[] = [];

            const windowEntries: [string, NanoGPTQuotaWindow | null | undefined, boolean][] = [
              ['weekly_input_tokens', qd.windows?.weeklyInputTokens, true],
              ['daily_images', qd.windows?.dailyImages, false],
              ['daily_input_tokens', qd.windows?.dailyInputTokens, true],
            ];

            windowEntries.forEach(([key, window, isTokens], idx) => {
              if (!window) return;
              const label = t(`quota.window.${key}`);
              const pct = (window.percentUsed ?? 0) * 100;
              const usedStr = isTokens ? formatTokenCount(window.used ?? 0) : `${window.used ?? 0}`;
              const remStr = isTokens ? formatTokenCount(window.remaining ?? 0) : `${window.remaining ?? 0}`;

              items.push(
                <div key={key} className={idx > 0 ? 'border-border/60 space-y-2.5 border-t border-dashed pt-3' : 'space-y-2.5'}>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {label}{' '}
                        <span className='font-normal opacity-70'>
                          ({usedStr}/{remStr})
                        </span>
                      </span>
                      <span className='text-foreground font-medium'>{Math.round(pct)}%</span>
                    </div>
                    <ProgressBar percentage={pct} />
                  </div>
                  {window.resetAt ? (
                    <div className='text-muted-foreground pt-0.5 text-right text-[11px]'>
                      {formatTimeToReset(new Date(window.resetAt).toISOString())}
                    </div>
                  ) : null}
                </div>
              );
            });

            if (qd.state && qd.state !== 'active') {
              const stateKey = `quota.label.state_${qd.state}`;
              items.push(
                <div key='state' className='flex items-center gap-1.5 pt-1'>
                  <Badge
                    variant='outline'
                    className='h-4 border-yellow-500/30 px-1.5 py-0 text-[10px] font-semibold tracking-wider text-yellow-500 uppercase'
                  >
                    {t(stateKey)}
                  </Badge>
                </div>
              );
            }

            return items;
          })()}
        </div>
      )}

      {channel.type === 'openai' && channel.providerType === 'wafer' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData as ProviderWaferQuotaData | undefined;
            if (!qd) return null;
            const items: React.ReactNode[] = [];

            const usedPct = qd.current_period_used_percent ?? 0;
            const usedRequests = (qd.included_request_limit ?? 0) - (qd.remaining_included_requests ?? 0);
            const totalRequests = qd.included_request_limit ?? 0;
            items.push(
              <div key='usage' className='space-y-2.5'>
                <div className='space-y-1'>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='text-muted-foreground font-medium'>
                      {t('quota.label.requests')}{' '}
                      <span className='font-normal opacity-70'>
                        ({usedRequests}/{totalRequests})
                      </span>
                    </span>
                    <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</span>
                  </div>
                  <ProgressBar percentage={usedPct} />
                </div>
              </div>
            );

            if (qd.window_end) {
              items.push(
                <div key='reset' className='text-muted-foreground pt-1 text-right text-[11px]'>
                  {formatTimeToReset(qd.window_end, usedPct)}
                </div>
              );
            }

            return items;
          })()}
        </div>
      )}

      {channel.type === 'openai' && channel.providerType === 'synthetic' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData as ProviderSyntheticQuotaData | undefined;
            if (!qd) return null;
            const items: React.ReactNode[] = [];

            if (qd.weeklyTokenLimit) {
              const pctRemaining = qd.weeklyTokenLimit.percentRemaining ?? 100;
              const usedPct = 100 - pctRemaining;
              const remainingCredits = qd.weeklyTokenLimit.remainingCredits;
              const maxCredits = qd.weeklyTokenLimit.maxCredits;
              const usedCredits =
                remainingCredits != null && maxCredits != null
                  ? `$${(parseFloat(maxCredits.replace('$', '')) - parseFloat(remainingCredits.replace('$', ''))).toFixed(2)}`
                  : null;
              items.push(
                <div key='weekly' className='space-y-2.5'>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t('quota.label.weekly_token_limit')}
                        {usedCredits != null && maxCredits != null && (
                          <span className='font-normal opacity-70'>
                            {' '}
                            ({usedCredits}/{maxCredits})
                          </span>
                        )}
                      </span>
                      <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</span>
                    </div>
                    <ProgressBar percentage={usedPct} />
                  </div>
                  {qd.weeklyTokenLimit.nextRegenAt && (
                    <div className='text-muted-foreground pt-0.5 text-right text-[11px]'>
                      {formatTimeToReset(qd.weeklyTokenLimit.nextRegenAt, usedPct, syntheticWeeklyRegenTickPct)}
                    </div>
                  )}
                </div>
              );
            }

            if (qd.rollingFiveHourLimit) {
              const fiveHrRemaining = qd.rollingFiveHourLimit.remaining ?? 0;
              const fiveHrMax = qd.rollingFiveHourLimit.max ?? 0;
              const fiveHrUsed = fiveHrMax - fiveHrRemaining;
              const fiveHrUsedPct = fiveHrMax > 0 ? (fiveHrUsed / fiveHrMax) * 100 : 0;
              items.push(
                <div key='5h' className='border-border/60 space-y-2.5 border-t border-dashed pt-3'>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t('quota.label.rolling_5h_limit')}{' '}
                        <span className='font-normal opacity-70'>
                          ({Math.round(fiveHrUsed)}/{Math.round(fiveHrMax)})
                        </span>
                      </span>
                      <span className='text-foreground font-medium'>
                        {t('quota.label.percent_used', { percent: Math.round(fiveHrUsedPct) })}
                      </span>
                    </div>
                    <ProgressBar percentage={fiveHrUsedPct} />
                  </div>
                  {qd.rollingFiveHourLimit.limited && (
                    <Badge
                      variant='outline'
                      className='h-4 border-yellow-500/30 px-1.5 py-0 text-[10px] font-semibold tracking-wider text-yellow-500 uppercase'
                    >
                      {t('quota.status.limited')}
                    </Badge>
                  )}
                  {qd.rollingFiveHourLimit.nextTickAt && (
                    <div className='text-muted-foreground pt-0.5 text-right text-[11px]'>
                      {formatTimeToReset(qd.rollingFiveHourLimit.nextTickAt, fiveHrUsedPct, qd.rollingFiveHourLimit.tickPercent ?? 0.05)}
                    </div>
                  )}
                </div>
              );
            }

            return items;
          })()}
        </div>
      )}

      {channel.type === 'openai' && channel.providerType === 'neuralwatt' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus?.quotaData as ProviderNeuralWattQuotaData | undefined;
            if (!qd) return null;
            const items: React.ReactNode[] = [];

            if (qd.subscription) {
              const kwhIncluded = qd.subscription.kwh_included ?? 0;
              const kwhUsed = qd.subscription.kwh_used ?? 0;
              const usedPct = kwhIncluded > 0 ? (kwhUsed / kwhIncluded) * 100 : 0;

              items.push(
                <div key='kwh' className='space-y-2.5'>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t('quota.label.kwh_remaining')}
                        <span className='font-normal opacity-70'>
                          {' '}
                          ({kwhUsed}/{kwhIncluded})
                        </span>
                      </span>
                      <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</span>
                    </div>
                    <ProgressBar percentage={usedPct} />
                  </div>
                </div>
              );

              if (qd.subscription.in_overage) {
                items.push(
                  <div key='overage' className='flex items-center gap-1.5 pt-1'>
                    <Badge variant='destructive' className='h-4 px-1.5 py-0 text-[10px] font-semibold tracking-wider uppercase'>
                      {t('quota.label.in_overage')}
                    </Badge>
                  </div>
                );
              }
            }

            if (qd.balance) {
              items.push(
                <div key='credits' className='border-border/60 space-y-2.5 border-t border-dashed pt-3'>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='text-muted-foreground font-medium'>{t('quota.label.credits_remaining')}</span>
                    <span className='text-foreground font-medium'>
                      {qd.balance.credits_remaining_usd != null ? `$${qd.balance.credits_remaining_usd.toFixed(2)}` : '$0.00'}
                    </span>
                  </div>
                </div>
              );
            }

            if (quota.nextResetAt) {
              items.push(
                <div key='reset' className='text-muted-foreground pt-1 text-right text-[11px]'>
                  {formatTimeToReset(quota.nextResetAt)}
                </div>
              );
            }

            return items;
          })()}
        </div>
      )}
    </div>
  );
}

function QuotaBadgeTrigger({ channels }: { channels: ProviderQuotaChannel[] }) {
  const highestUsed = Math.max(
    ...channels.map((c) => {
      const quota = c.quotaStatus;
      if (!quota) return 0;
      return getChannelPercentage(c);
    })
  );

  const hasExhausted = channels.some((c) => c.quotaStatus?.status === 'exhausted');
  const hasWarning = channels.some((c) => c.quotaStatus?.status === 'warning');

  let level: BatteryLevel = 'full';
  if (hasExhausted) level = 'warning';
  else if (hasWarning) level = 'low';
  else level = getBatteryLevel(highestUsed, 'available');

  const BatteryIcon = getBatteryIcon(level);
  const isWarning = level === 'warning';
  const textColor = isWarning ? 'text-red-500' : level === 'low' ? 'text-yellow-500' : 'text-muted-foreground';

  return <BatteryIcon className={`h-5 w-5 ${textColor} transition-colors`} />;
}

export function QuotaBadges({ isRefreshing, onRefresh }: { isRefreshing: boolean; onRefresh: () => void }) {
  const { t } = useTranslation();
  const channels = useProviderQuotaStatuses();
  const { data: enforcementSettings } = useQuotaEnforcementSettings();
  const enforcementMode = enforcementSettings?.enabled ? enforcementSettings.mode : null;

  if (channels.length === 0) return null;

  const groupedChannels = channels.reduce((acc: ProviderQuotaChannel[], channel: ProviderQuotaChannel) => {
    if (channel.type === 'nanogpt_responses') {
      const existing = acc.find((c) => c.type === 'nanogpt');
      if (!existing) {
        acc.push(channel);
      }
    } else if (channel.type === 'openai' && channel.providerType) {
      const existing = acc.find((c) => c.type === 'openai' && c.providerType === channel.providerType);
      if (!existing) {
        acc.push(channel);
      }
    } else {
      acc.push(channel);
    }
    return acc;
  }, [] as ProviderQuotaChannel[]);

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button type='button' className='hover:bg-muted relative rounded-md p-2 transition-colors'>
          <QuotaBadgeTrigger channels={groupedChannels} />
        </button>
      </PopoverTrigger>
      <PopoverContent className={groupedChannels.length > 4 ? 'w-full sm:w-[640px]' : 'w-full sm:w-80'} align='end'>
        <div className='space-y-1'>
          <div className='mb-2 flex items-center justify-between'>
            <div className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>{t('system.providerQuota.title')}</div>
            <button
              onClick={onRefresh}
              disabled={isRefreshing}
              className='text-muted-foreground hover:text-foreground transition-colors'
              aria-label={t('system.providerQuota.refresh.label')}
            >
              {isRefreshing ? <Loader2 className='h-4 w-4 animate-spin' /> : <RefreshCw className='h-4 w-4' />}
            </button>
          </div>
          <div
            className={`max-h-[60vh] overflow-y-auto pr-1 pl-1 ${groupedChannels.length > 4 ? 'grid grid-cols-1 gap-x-4 sm:grid-cols-2' : ''}`}
          >
            {groupedChannels.map((channel: ProviderQuotaChannel) => (
              <QuotaRow key={channel.id} channel={channel} enforcementMode={enforcementMode} />
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
