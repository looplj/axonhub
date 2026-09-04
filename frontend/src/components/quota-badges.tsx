import { useState, type ReactNode } from 'react';
import { format } from 'date-fns';
import { useQueryClient } from '@tanstack/react-query';
import { Loader2, RefreshCw, Zap, Battery, BatteryLow, BatteryMedium, BatteryFull, BatteryWarning } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  useProviderQuotaStatuses,
  ProviderQuotaChannel,
  ProviderQuotaLimit,
  ProviderNanoGPTQuotaData,
  NanoGPTQuotaWindow,
  ProviderWaferQuotaData,
  ProviderSyntheticQuotaData,
  ProviderNeuralWattQuotaData,
  ProviderApertisQuotaData,
  ProviderCharmHyperQuotaData,
  ProviderOpenCodeGoQuotaData,
  OpenCodeGoQuotaWindow,
  ProviderKimiCodeQuotaData,
  ProviderMinimaxQuotaData,
  ProviderZhipuQuotaData,
  ProviderZenmuxQuotaData,
  ClineQuotaWindow,
  isClineActivePassQuotaData,
  isClineUnavailablePassQuotaData,
  resetChannelQuotaNow,
  checkProviderQuotas,
} from '@/features/system/data/quotas';
import { useGeneralSettings, useQuotaEnforcementSettings, type QuotaEnforcementMode } from '@/features/system/data/system';
import { capitalizeZenmuxTier, getZenmuxMonthlyQuotaUSD, getZenmuxUsagePercentage } from '@/features/system/data/zenmux-quota-display';

const syntheticWeeklyRegenTickPct = 0.02;

const BADGE_COLOR_CLASSES: Record<string, string> = {
  green: 'bg-green-500/10 text-green-500 border-green-500/20 hover:bg-green-500/20',
  red: 'bg-red-500/10 text-red-500 border-red-500/20 hover:bg-red-500/20',
  amber: 'bg-amber-500/10 text-amber-500 border-amber-500/20 hover:bg-amber-500/20',
  blue: 'bg-blue-500/10 text-blue-500 border-blue-500/20 hover:bg-blue-500/20',
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

function isOpenaiType(t: string): t is 'openai' | 'openai_responses' {
  return t === 'openai' || t === 'openai_responses';
}

function isOpenCodeGoType(t: string): t is 'opencode_go' | 'opencode_go_anthropic' {
  return t === 'opencode_go' || t === 'opencode_go_anthropic';
}

function isMinimaxType(t: string): t is 'minimax' | 'minimax_anthropic' {
  return t === 'minimax' || t === 'minimax_anthropic';
}

function isZenmuxType(t: string): t is 'zenmux' | 'zenmux_responses' | 'zenmux_anthropic' | 'zenmux_gemini' {
  return t === 'zenmux' || t === 'zenmux_responses' || t === 'zenmux_anthropic' || t === 'zenmux_gemini';
}

// Fraction of a limit's reset window that has elapsed, derived from the
// PeriodStart/NextResetAt pair the backend stamps on windowed limits.
function getLimitDurationPercent(limit: ProviderQuotaLimit): number | undefined {
  if (!limit.periodStart || !limit.nextResetAt) return undefined;
  const start = new Date(limit.periodStart).getTime();
  const end = new Date(limit.nextResetAt).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return undefined;
  return Math.max(0, Math.min(100, ((Date.now() - start) / (end - start)) * 100));
}

function getClineUsagePercent(window?: ClineQuotaWindow): number {
  return window?.usage_percent ?? (window?.usage_ratio ?? 0) * 100;
}

function getChannelPercentage(channel: ProviderQuotaChannel): number {
  let percentage = 0;
  if (channel.type === 'claudecode') {
    const qd = channel.quotaStatus.quotaData;
    const util5h = qd.windows?.['5h']?.utilization || 0;
    const util7d = qd.windows?.['7d']?.utilization || 0;
    percentage = Math.max(util5h, util7d) * 100;
  } else if (channel.type === 'codex') {
    const qd = channel.quotaStatus.quotaData;
    percentage = qd.rate_limit?.primary_window?.used_percent || 0;
  } else if (channel.type === 'xai_subscription') {
    const qd = channel.quotaStatus.quotaData;
    percentage = Math.max(qd.billing?.weekly?.usage_percent ?? 0, qd.billing?.monthly?.usage_percent ?? 0);
  } else if (channel.type === 'cline') {
    const qd = channel.quotaStatus.quotaData;
    percentage = isClineActivePassQuotaData(qd)
      ? Math.max(getClineUsagePercent(qd.windows.last5h), getClineUsagePercent(qd.windows.last7d), getClineUsagePercent(qd.windows.last30d))
      : 0;
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
    const qd = channel.quotaStatus.quotaData;
    let maxPercent = 0;
    if (qd.windows?.weeklyInputTokens) maxPercent = Math.max(maxPercent, (qd.windows.weeklyInputTokens.percentUsed ?? 0) * 100);
    if (qd.windows?.dailyInputTokens) maxPercent = Math.max(maxPercent, (qd.windows.dailyInputTokens.percentUsed ?? 0) * 100);
    if (qd.windows?.dailyImages) maxPercent = Math.max(maxPercent, (qd.windows.dailyImages.percentUsed ?? 0) * 100);
    percentage = maxPercent;
  } else if (isOpenCodeGoType(channel.type)) {
    const qd = channel.quotaStatus.quotaData as ProviderOpenCodeGoQuotaData | undefined;
    percentage = Math.max(
      qd?.windows?.rolling?.usage_percent ?? 0,
      qd?.windows?.weekly?.usage_percent ?? 0,
      qd?.windows?.monthly?.usage_percent ?? 0
    );
  } else if (channel.type === 'moonshot_coding') {
    const qd = channel.quotaStatus.quotaData as ProviderKimiCodeQuotaData | undefined;
    percentage = Math.max(0, ...(qd?.rows ?? []).map((row) => (row.limit > 0 ? (row.used / row.limit) * 100 : 0)));
  } else if (channel.type === 'minimax' || channel.type === 'minimax_anthropic') {
    const qd = channel.quotaStatus.quotaData as ProviderMinimaxQuotaData | undefined;
    percentage = Math.max(
      0,
      ...(qd?.rows ?? []).map((row) => Math.max(row.intervalPercent, row.weeklyPercent))
    );
  } else if (channel.type === 'zhipu' || channel.type === 'zhipu_anthropic') {
    const qd = channel.quotaStatus.quotaData as ProviderZhipuQuotaData | undefined;
    percentage = Math.max(0, ...(qd?.rows ?? []).map((row) => row.usedPercent));
  } else if (isOpenaiType(channel.type) && channel.providerType === 'wafer') {
    const qd = channel.quotaStatus.quotaData as ProviderWaferQuotaData | undefined;
    percentage = qd?.current_period_used_percent ?? 0;
  } else if (isOpenaiType(channel.type) && channel.providerType === 'synthetic') {
    const qd = channel.quotaStatus.quotaData as ProviderSyntheticQuotaData | undefined;
    const weeklyPct = qd?.weeklyTokenLimit?.percentRemaining ?? 100;
    percentage = 100 - weeklyPct;
  } else if (isOpenaiType(channel.type) && channel.providerType === 'neuralwatt') {
    const qd = channel.quotaStatus.quotaData as ProviderNeuralWattQuotaData | undefined;
    const kwhIncluded = qd?.subscription?.kwh_included ?? 0;
    const kwhUsed = qd?.subscription?.kwh_used ?? 0;
    if (kwhIncluded > 0) {
      percentage = (kwhUsed / kwhIncluded) * 100;
    }
  } else if (isOpenaiType(channel.type) && channel.providerType === 'apertis') {
    percentage = getApertisPercentage(channel.quotaStatus.quotaData as ProviderApertisQuotaData | undefined);
  } else if (isOpenaiType(channel.type) && channel.providerType === 'charm_hyper') {
    const qd = channel.quotaStatus.quotaData as ProviderCharmHyperQuotaData | undefined;
    const balance = qd?.balance;
    if (typeof balance === 'number') {
      percentage = Math.max(0, Math.min(100, (1.0 - balance / 100) * 100));
    }
  } else if (isZenmuxType(channel.type)) {
    percentage = getZenmuxUsagePercentage(channel.quotaStatus.limits);
  }
  return percentage;
}
function getApertisPercentage(qd: ProviderApertisQuotaData | undefined): number {
  if (!qd) return 0;
  if (qd.is_subscriber && qd.subscription?.cycle_quota_limit) {
    return (qd.subscription.cycle_quota_used / qd.subscription.cycle_quota_limit) * 100;
  }
  if (qd.payg && !qd.payg.token_is_unlimited && typeof qd.payg.token_total === 'number' && typeof qd.payg.token_used === 'number') {
    return (qd.payg.token_used / qd.payg.token_total) * 100;
  }
  return 0;
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

// UsageTimeBar shows usage on a single progress bar with a small triangle below
// it marking how far the reset window has elapsed (time progress). Hovering
// reveals the detailed figures via tooltip, keeping the row compact.
function UsageTimeBar({ usagePercent, durationPercent, tooltip }: { usagePercent: number; durationPercent?: number; tooltip: ReactNode }) {
  const markerLeft = durationPercent === undefined ? undefined : Math.min(Math.max(durationPercent, 0), 100);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='relative cursor-default pb-1.5'>
          <ProgressBar percentage={usagePercent} durationPercentage={durationPercent} />
          {markerLeft !== undefined && (
            <div className='absolute top-2 -translate-x-1/2' style={{ left: `${markerLeft}%` }} aria-hidden>
              {/* upward triangle pointing at the bar, marking elapsed time */}
              <div className='border-b-muted-foreground h-0 w-0 border-x-[3px] border-b-[4px] border-x-transparent' />
            </div>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent side='top'>{tooltip}</TooltipContent>
    </Tooltip>
  );
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return `${n}`;
}

const WINDOW_LABEL_KEYS: Record<string, string> = {
  '5h': 'quota.window.5h',
  '7d': 'quota.window.7d',
  '30d': 'quota.window.30d',
  daily: 'quota.window.daily',
  weekly: 'quota.window.weekly',
  monthly: 'quota.window.monthly',
  primary: 'quota.label.primary_window',
  secondary: 'quota.label.secondary_window',
  overage: 'quota.label.overage_window',
  cycle: 'quota.label.subscription',
};

// PeriodQuotaEstimate prices each limit window: the backend sums what the
// channel cost during the window from AxonHub usage logs and divides by the
// usage ratio the provider reported, which yields what the whole window is
// worth. Windows the backend could not price are simply absent.
function PeriodQuotaEstimate({ limits }: { limits: ProviderQuotaLimit[] }) {
  const { t, i18n } = useTranslation();
  const { data: generalSettings } = useGeneralSettings();

  const priced = limits.filter((limit) => limit.periodQuota != null);
  if (priced.length === 0) return null;

  const formatCurrency = (val: number) =>
    t('currencies.format', {
      val,
      currency: generalSettings?.currencyCode || 'USD',
      locale: i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });

  return (
    <div className='border-border/60 mt-3 space-y-2 border-t border-dashed pt-3'>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className='text-muted-foreground cursor-default text-xs font-medium'>{t('quota.label.period_quota')}</span>
        </TooltipTrigger>
        <TooltipContent side='top' className='max-w-[260px]'>
          {t('quota.label.period_quota_hint')}
        </TooltipContent>
      </Tooltip>

      {priced.map((limit, index) => {
        const labelKey = limit.window ? WINDOW_LABEL_KEYS[limit.window] : undefined;
        const label = labelKey ? t(labelKey) : limit.window || t('quota.label.token_usage');

        return (
          <div key={`${limit.window ?? limit.type}-${index}`} className='flex items-center justify-between text-xs'>
            <span className='text-muted-foreground'>{label}</span>
            <span className='text-foreground font-medium'>
              {t('quota.label.period_quota_value', {
                // The backend only estimates a quota once it has a cost, so
                // both figures are always available here.
                used: formatCurrency(limit.periodCost ?? 0),
                total: formatCurrency(limit.periodQuota as number),
              })}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function QuotaRow({ channel, enforcementMode, allowedChannelIDs }: { channel: ProviderQuotaChannel; enforcementMode?: QuotaEnforcementMode | null; allowedChannelIDs?: string[] | null }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [isResetting, setIsResetting] = useState(false);
  const quota = channel.quotaStatus;

  const status = quota.status;
  const clinePassUnavailable = channel.type === 'cline' && isClineUnavailablePassQuotaData(channel.quotaStatus.quotaData);
  const statusLabel = clinePassUnavailable ? t('quota.status.cline_pass_unavailable') : t(STATUS_LABELS[status]);

  const isAllowed = allowedChannelIDs?.includes(channel.id) ?? false;

  const enforcementEffect =
    enforcementMode && !isAllowed && (status === 'exhausted' || (status === 'warning' && enforcementMode === 'DE_PRIORITIZE'))
      ? enforcementMode === 'EXHAUSTED_ONLY'
        ? ('blocked' as const)
        : ('deprioritized' as const)
      : null;

  const percentage = getChannelPercentage(channel);
  const batteryLevel = getBatteryLevel(percentage, status);
  const BatteryIcon = getBatteryIcon(batteryLevel);

  const handleResetCodexQuota = async () => {
    if (channel.type !== 'codex') return;

    setIsResetting(true);
    try {
      await resetChannelQuotaNow(channel.id);
      toast.success(t('quota.codex.resetSuccess'));
      // Trigger a backend quota refresh, then refetch the cached statuses.
      await checkProviderQuotas();
      await queryClient.invalidateQueries({ queryKey: ['provider-quotas'] });
    } catch (err) {
      toast.error(t('quota.codex.resetError'), {
        description: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsResetting(false);
    }
  };

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

  // Fraction of the reset window that has elapsed, rendered as a second
  // "time elapsed" bar so usage progress can be read against time progress.
  const getOpenCodeGoDurationPercent = (key: 'rolling' | 'weekly' | 'monthly', window: OpenCodeGoQuotaWindow): number | undefined => {
    const limits: Record<string, number> = {
      rolling: 5 * 3600,
      weekly: 7 * 24 * 3600,
      monthly: 30 * 24 * 3600,
    };
    const limit = limits[key];

    // Prefer the absolute reset_time so the marker keeps advancing as the user
    // looks; fall back to the snapshot reset_in_seconds from the last poll.
    let resetAfter: number | undefined;
    if (window.reset_time) {
      resetAfter = (new Date(window.reset_time).getTime() - Date.now()) / 1000;
    } else if (window.reset_in_seconds != null) {
      resetAfter = window.reset_in_seconds;
    }
    if (resetAfter === undefined) return undefined;

    return calcDurationPercent(limit, resetAfter);
  };

  const getClineDurationPercent = (key: 'last5h' | 'last7d' | 'last30d', window: ClineQuotaWindow): number | undefined => {
    const limits: Record<'last5h' | 'last7d' | 'last30d', number> = {
      last5h: 5 * 3600,
      last7d: 7 * 24 * 3600,
      last30d: 30 * 24 * 3600,
    };
    if (!window.next_reset_at) return undefined;
    const resetAfter = (new Date(window.next_reset_at).getTime() - Date.now()) / 1000;
    return calcDurationPercent(limits[key], resetAfter);
  };

  const formatClineCost = (units?: number, scale?: number) => {
    if (units == null || !scale) return '';
    return t('currencies.format', {
      val: units / scale,
      currency: 'USD',
      locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
      minimumFractionDigits: 6,
    });
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

  const formatQuotaResetTime = (value?: string) => {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : format(date, 'yyyy-MM-dd HH:mm');
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
            variant={status === 'exhausted' ? 'destructive' : 'outline'}
            className={status === 'available' ? BADGE_COLOR_CLASSES.green : status === 'warning' ? BADGE_COLOR_CLASSES.amber : ''}
          >
            {statusLabel}
          </Badge>
          {enforcementEffect && (
            <Badge variant='outline' className={BADGE_COLOR_CLASSES[enforcementEffect === 'blocked' ? 'red' : 'amber']}>
              {t(`quota.status.${enforcementEffect}`)}
            </Badge>
          )}
          {isAllowed && (status === 'exhausted' || status === 'warning') && (
            <Badge variant='outline' className={BADGE_COLOR_CLASSES.blue}>
              {t('quota.status.bypassed')}
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
            const qd = channel.quotaStatus.quotaData;
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
                    {qd.windows['7d'].reset && (
                      <div className='text-muted-foreground pt-0.5 text-right text-[11px]'>
                        {formatTimeToReset(new Date(qd.windows['7d'].reset * 1000).toISOString())} (
                        {formatDate(qd.windows['7d'].reset)})
                      </div>
                    )}
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
            const qd = channel.quotaStatus.quotaData;
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
            const qd = channel.quotaStatus.quotaData;
            if (!qd) return null;
            const availableResets = qd._resets?.resets ?? [];
            const availableResetCount = availableResets.length;
            const nextExpiringReset = availableResets
              .filter((reset) => reset.expiresAt && !Number.isNaN(new Date(reset.expiresAt).getTime()))
              .sort((a, b) => new Date(a.expiresAt!).getTime() - new Date(b.expiresAt!).getTime())[0];
            const latestGrantedReset = availableResets
              .filter((reset) => reset.grantedAt && !Number.isNaN(new Date(reset.grantedAt).getTime()))
              .sort((a, b) => new Date(b.grantedAt!).getTime() - new Date(a.grantedAt!).getTime())[0];
            const resetTime = nextExpiringReset?.expiresAt ?? latestGrantedReset?.grantedAt;
            const hasResetInfo = qd._resets?.supported === true && !qd._resets.error;
            const canAttemptReset =
              qd._resets?.supported === true && (Boolean(qd._resets.error) || availableResetCount > 0);
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

                <div className='border-border/60 space-y-2 border-t border-dashed pt-3'>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='text-muted-foreground font-medium'>{t('quota.codex.availableResets')}</span>
                    <span className='text-foreground font-medium'>
                      {hasResetInfo
                        ? t('quota.codex.availableResetsCount', { count: availableResetCount })
                        : t('quota.label.unavailable')}
                    </span>
                  </div>
                  {resetTime && (
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {nextExpiringReset ? t('quota.codex.expiresAt') : t('quota.codex.grantedAt')}
                      </span>
                      <span className='text-foreground font-medium'>{formatQuotaResetTime(resetTime)}</span>
                    </div>
                  )}
                  {(status === 'exhausted' || status === 'warning') && (
                    <div className='flex items-center justify-end pt-1'>
                      <Button
                        size='sm'
                        variant='outline'
                        className='h-7 text-xs'
                        disabled={isResetting || !canAttemptReset}
                        title={!canAttemptReset ? t('quota.codex.noResetCredits') : undefined}
                        onClick={handleResetCodexQuota}
                      >
                        {isResetting ? <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' /> : <Zap className='mr-1.5 h-3.5 w-3.5' />}
                        {t('quota.codex.resetNow')}
                      </Button>
                    </div>
                  )}
                </div>
              </>
            );
          })()}
        </div>
      )}

      {channel.type === 'xai_subscription' && (
        <div className='mt-4 space-y-4'>
          {(() => {
            const qd = channel.quotaStatus.quotaData;
            const weekly = qd.billing?.weekly;
            const monthly = qd.billing?.monthly;
            return (
              <>
                {qd.plan_type ? (
                  <div className='text-muted-foreground flex items-center justify-between text-xs'>
                    <span>{t('quota.label.plan')}</span>
                    <span className='text-foreground font-medium'>{qd.plan_type}</span>
                  </div>
                ) : null}
                {weekly ? (
                  <div className='space-y-1.5'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>{t('quota.window.weekly')}</span>
                      <span className='text-foreground font-medium'>
                        {t('quota.label.percent_used', { percent: Math.round(weekly.usage_percent ?? 0) })}
                      </span>
                    </div>
                    <ProgressBar percentage={weekly.usage_percent ?? 0} />
                    {weekly.reset_at ? (
                      <div className='text-muted-foreground text-right text-[11px]'>{formatTimeToReset(weekly.reset_at)}</div>
                    ) : null}
                  </div>
                ) : null}
                {monthly ? (
                  <div className='border-border/60 space-y-1.5 border-t border-dashed pt-3'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t('quota.window.monthly')}{' '}
                        {monthly.limit_usd ? (
                          <span className='font-normal opacity-70'>
                            (
                            {t('currencies.format', {
                              val: monthly.used_usd ?? 0,
                              currency: 'USD',
                              locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                              minimumFractionDigits: 2,
                            })}
                            {' / '}
                            {t('currencies.format', {
                              val: monthly.limit_usd,
                              currency: 'USD',
                              locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                              minimumFractionDigits: 2,
                            })}
                            )
                          </span>
                        ) : null}
                      </span>
                      <span className='text-foreground font-medium'>
                        {t('quota.label.percent_used', { percent: Math.round(monthly.usage_percent ?? 0) })}
                      </span>
                    </div>
                    <ProgressBar percentage={monthly.usage_percent ?? 0} />
                    {monthly.reset_at ? (
                      <div className='text-muted-foreground text-right text-[11px]'>{formatTimeToReset(monthly.reset_at)}</div>
                    ) : null}
                  </div>
                ) : null}
              </>
            );
          })()}
        </div>
      )}

      {channel.type === 'cline' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData;
            const items: React.ReactNode[] = [];

            if (isClineUnavailablePassQuotaData(qd)) {
              items.push(
                <div key='pass-unavailable' className='text-muted-foreground bg-muted/40 rounded p-2 text-[11px]'>
                  {t('quota.label.cline_pass_unavailable')}
                </div>
              );
            }

            if (isClineActivePassQuotaData(qd)) {
              const entries: Array<['last5h' | 'last7d' | 'last30d', string]> = [
                ['last5h', 'quota.window.5h'],
                ['last7d', 'quota.window.7d'],
                ['last30d', 'quota.window.30d'],
              ];

              entries.forEach(([key, labelKey], index) => {
                const window = qd.windows[key];
                const usedPct = getClineUsagePercent(window);
                const hasUsagePercent = window.usage_percent != null || window.usage_ratio != null;
                const durationPct = getClineDurationPercent(key, window);
                const used = formatClineCost(window.used_cost_units, qd.cost_scale);
                const limit = formatClineCost(window.limit_cost_units, qd.cost_scale);
                const resetText = window.next_reset_at ? formatTimeToReset(window.next_reset_at) : '';

                items.push(
                  <div key={key} className={index > 0 ? 'border-border/60 space-y-1.5 border-t border-dashed pt-3' : 'space-y-1.5'}>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t(labelKey)}{' '}
                        {used && limit ? (
                          <span className='font-normal opacity-70'>
                            ({used}/{limit})
                          </span>
                        ) : null}
                      </span>
                      <span className='text-foreground font-medium'>
                        {hasUsagePercent ? t('quota.label.percent_used', { percent: Math.round(usedPct) }) : t('quota.label.unavailable')}
                      </span>
                    </div>
                    {hasUsagePercent ? (
                      <UsageTimeBar
                        usagePercent={usedPct}
                        durationPercent={durationPct}
                        tooltip={
                          <div className='space-y-0.5'>
                            <div className='font-medium'>{t(labelKey)}</div>
                            <div>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</div>
                            {durationPct !== undefined && (
                              <div>
                                {t('quota.label.time_elapsed')}: {Math.round(durationPct)}%
                              </div>
                            )}
                            {resetText && <div>{resetText}</div>}
                          </div>
                        }
                      />
                    ) : (
                      <div className='text-muted-foreground text-[11px]'>{t('quota.label.unavailable')}</div>
                    )}
                  </div>
                );
              });
            }

            if (qd.balance?.raw_balance != null) {
              items.push(
                <div key='balance' className='border-border/60 flex items-center justify-between border-t border-dashed pt-3 text-xs'>
                  <span className='text-muted-foreground font-medium'>{t('quota.label.cline_balance')}</span>
                  <span className='text-foreground font-medium'>{qd.balance.raw_balance.toLocaleString()}</span>
                </div>
              );
            } else if (qd.model_scope === 'direct_only') {
              items.push(
                <div key='balance-unavailable' className='border-border/60 flex items-center justify-between border-t border-dashed pt-3 text-xs'>
                  <span className='text-muted-foreground font-medium'>{t('quota.label.cline_balance')}</span>
                  <span className='text-muted-foreground'>{t('quota.label.unavailable')}</span>
                </div>
              );
            }

            if (qd.model_scope === 'mixed') {
              items.push(
                <div key='mixed-note' className='text-muted-foreground bg-muted/40 rounded p-2 text-[11px]'>
                  {t('quota.label.cline_mixed_pool_note')}
                </div>
              );
            }

            if (qd.model_scope === 'direct_only') {
              items.push(
                <div key='direct-note' className='text-muted-foreground bg-muted/40 rounded p-2 text-[11px]'>
                  {t('quota.label.cline_direct_pool_note')}
                </div>
              );
            }

            if (items.length === 0) {
              items.push(
                <div key='unavailable' className='text-muted-foreground bg-muted/40 rounded p-2 text-[11px]'>
                  {t('quota.label.unavailable')}
                </div>
              );
            }

            return items;
          })()}
        </div>
      )}

      {(channel.type === 'nanogpt' || channel.type === 'nanogpt_responses') && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderNanoGPTQuotaData | undefined;
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
              const total = (window.used ?? 0) + (window.remaining ?? 0);
              const totalStr = isTokens ? formatTokenCount(total) : `${total}`;

              items.push(
                <div key={key} className={idx > 0 ? 'border-border/60 space-y-2.5 border-t border-dashed pt-3' : 'space-y-2.5'}>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {label}{' '}
                        <span className='font-normal opacity-70'>
                          ({usedStr}/{totalStr})
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

      {isOpenCodeGoType(channel.type) && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderOpenCodeGoQuotaData | undefined;
            if (!qd) return null;

            const entries: Array<['rolling' | 'weekly' | 'monthly', string]> = [
              ['rolling', 'quota.window.5h'],
              ['weekly', 'quota.window.weekly'],
              ['monthly', 'quota.window.monthly'],
            ];

            return entries
              .map(([key, labelKey], index) => {
                const window = qd.windows?.[key];
                if (!window) return null;

                const usedPct = window.usage_percent ?? 0;
                const durationPct = getOpenCodeGoDurationPercent(key, window);
                // Always show the reset countdown — the elapsed-time marker already
                // conveys progress, so the "no usage yet" special case is unwanted here.
                const resetText =
                  window.reset_time || window.reset_in_seconds != null
                    ? formatTimeToReset(window.reset_time ?? window.reset_in_seconds)
                    : '';
                return (
                  <div key={key} className={index > 0 ? 'border-border/60 space-y-1.5 border-t border-dashed pt-3' : 'space-y-1.5'}>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>{t(labelKey)}</span>
                      <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</span>
                    </div>
                    <UsageTimeBar
                      usagePercent={usedPct}
                      durationPercent={durationPct}
                      tooltip={
                        <div className='space-y-0.5'>
                          <div className='font-medium'>{t(labelKey)}</div>
                          <div>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</div>
                          {durationPct !== undefined && (
                            <div>
                              {t('quota.label.time_elapsed')}: {Math.round(durationPct)}%
                            </div>
                          )}
                          {resetText && <div>{resetText}</div>}
                        </div>
                      }
                    />
                  </div>
                );
              })
              .filter(Boolean);
          })()}
        </div>
      )}

      {channel.type === 'moonshot_coding' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderKimiCodeQuotaData | undefined;
            if (!qd) return null;

            const rows = qd.rows ?? [];
            const wallet = qd.boosterWallet;

            return (
              <>
                {rows.map((row, index) => {
                  const percentage = row.limit > 0 ? Math.min(100, (row.used / row.limit) * 100) : 0;
                  return (
                    <div key={`${row.label}-${index}`} className={index > 0 ? 'border-border/60 space-y-1.5 border-t border-dashed pt-3' : 'space-y-1.5'}>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>
                          {row.label}{' '}
                          <span className='font-normal opacity-70'>
                            ({row.used.toLocaleString()}/{row.limit.toLocaleString()})
                          </span>
                        </span>
                        <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(percentage) })}</span>
                      </div>
                      <ProgressBar percentage={percentage} />
                      {(row.resetAt || row.resetAfterSeconds) && (
                        <div className='text-muted-foreground text-right text-[11px]'>
                          {formatTimeToReset(row.resetAt ?? row.resetAfterSeconds)}
                        </div>
                      )}
                    </div>
                  );
                })}
                {wallet && (
                  <div className='border-border/60 space-y-1.5 border-t border-dashed pt-3 text-xs'>
                    <div className='flex items-center justify-between'>
                      <span className='text-muted-foreground font-medium'>{t('quota.label.booster_balance')}</span>
                      <span className='text-foreground font-medium'>
                        {t('currencies.format', {
                          val: wallet.balanceCents / 100,
                          currency: wallet.currency,
                          locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                          minimumFractionDigits: 2,
                        })}
                      </span>
                    </div>
                    {wallet.monthlyChargeLimitEnabled && (
                      <div className='flex items-center justify-between'>
                        <span className='text-muted-foreground'>{t('quota.label.monthly_spend')}</span>
                        <span className='text-foreground'>
                          {t('currencies.format', {
                            val: wallet.monthlyUsedCents / 100,
                            currency: wallet.currency,
                            locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                            minimumFractionDigits: 2,
                          })}
                          {' / '}
                          {wallet.monthlyChargeLimitCents > 0
                            ? t('currencies.format', {
                                val: wallet.monthlyChargeLimitCents / 100,
                                currency: wallet.currency,
                                locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                                minimumFractionDigits: 2,
                              })
                            : t('quota.label.unlimited')}
                        </span>
                      </div>
                    )}
                  </div>
                )}
              </>
            );
          })()}
        </div>
      )}

      {isMinimaxType(channel.type) && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderMinimaxQuotaData | undefined;
            if (!qd) return null;

            const rows = qd.rows ?? [];

            return (
              <>
                {rows.map((row, index) => {
                  const intervalUsed = Math.round(row.intervalUsedPercent);
                  const intervalTotal = Math.round(row.intervalTotalPercent);
                  const showIntervalTotal = intervalTotal !== 100;

                  const hasWeekly = row.weeklyStatus && row.weeklyStatus !== '';
                  const weeklyUsed = Math.round(row.weeklyUsedPercent);
                  const weeklyTotal = Math.round(row.weeklyTotalPercent);
                  const showWeeklyTotal = weeklyTotal !== 100;

                  return (
                    <div key={`${row.modelName}-${index}`} className={index > 0 ? 'border-border/60 space-y-3 border-t border-dashed pt-3' : 'space-y-3'}>
                      {rows.length > 1 && (
                        <div className='text-muted-foreground text-[11px] font-medium tracking-wide uppercase'>
                          {row.modelName}
                        </div>
                      )}
                      <div className='space-y-1.5'>
                        <div className='flex items-center justify-between text-xs'>
                          <span className='text-muted-foreground font-medium'>
                            {t('quota.window.5h')}
                          </span>
                          <span className='text-foreground font-medium'>
                            {showIntervalTotal
                              ? `${intervalUsed}% / ${intervalTotal}%`
                              : t('quota.label.percent_used', { percent: intervalUsed })}
                          </span>
                        </div>
                        <ProgressBar percentage={row.intervalPercent} />
                        {row.intervalResetAt && (
                          <div className='text-muted-foreground text-right text-[11px]'>
                            {formatTimeToReset(row.intervalResetAt)}
                          </div>
                        )}
                      </div>
                      {hasWeekly && (
                        <div className='border-border/60 space-y-1.5 border-t border-dashed pt-3'>
                          <div className='flex items-center justify-between text-xs'>
                            <span className='text-muted-foreground font-medium'>
                              {t('quota.window.weekly')}
                            </span>
                            <span className='text-foreground font-medium'>
                              {showWeeklyTotal
                                ? `${weeklyUsed}% / ${weeklyTotal}%`
                                : t('quota.label.percent_used', { percent: weeklyUsed })}
                            </span>
                          </div>
                          <ProgressBar percentage={row.weeklyPercent} />
                          {row.weeklyResetAt && (
                            <div className='text-muted-foreground text-right text-[11px]'>
                              {formatTimeToReset(row.weeklyResetAt)}
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </>
            );
          })()}
        </div>
      )}

      {(channel.type === 'zhipu' || channel.type === 'zhipu_anthropic') && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderZhipuQuotaData | undefined;
            if (!qd) return null;

            const rows = qd.rows ?? [];
            const windowLabels: Record<string, string> = {
              five_hour: t('quota.window.5h'),
              weekly_limit: t('quota.window.weekly'),
            };

            return (
              <>
                {rows.map((row, index) => {
                  const percentage = Math.min(100, row.usedPercent);
                  return (
                    <div key={`${row.window}-${index}`} className={index > 0 ? 'border-border/60 space-y-1.5 border-t border-dashed pt-3' : 'space-y-1.5'}>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>
                          {windowLabels[row.window] ?? row.window}
                        </span>
                        <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(percentage) })}</span>
                      </div>
                      <ProgressBar percentage={percentage} />
                      {row.resetAt && (
                        <div className='text-muted-foreground text-right text-[11px]'>
                          {formatTimeToReset(row.resetAt)}
                        </div>
                      )}
                    </div>
                  );
                })}
              </>
            );
          })()}
        </div>
      )}

      {isOpenaiType(channel.type) && channel.providerType === 'wafer' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderWaferQuotaData | undefined;
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

      {isOpenaiType(channel.type) && channel.providerType === 'synthetic' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderSyntheticQuotaData | undefined;
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

      {isOpenaiType(channel.type) && channel.providerType === 'neuralwatt' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderNeuralWattQuotaData | undefined;
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

      {isOpenaiType(channel.type) && channel.providerType === 'apertis' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderApertisQuotaData | undefined;
            if (!qd) return null;
            const items: React.ReactNode[] = [];

            // Subscription cycle quota (takes priority if subscriber)
            if (qd.is_subscriber && qd.subscription && qd.subscription.cycle_quota_limit > 0) {
              const subUsed = qd.subscription.cycle_quota_used;
              const subTotal = qd.subscription.cycle_quota_limit;
              const subPct = (subUsed / subTotal) * 100;
              const planLabel = qd.subscription.plan_type
                ? `${qd.subscription.plan_type.charAt(0).toUpperCase() + qd.subscription.plan_type.slice(1)} Plan`
                : t('quota.label.subscription');

              items.push(
                <div key='subscription' className='space-y-2.5'>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {planLabel}
                        <span className='font-normal opacity-70'>
                          {' '}
                          ({subUsed}/{subTotal})
                        </span>
                      </span>
                      <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(subPct) })}</span>
                    </div>
                    <ProgressBar percentage={subPct} />
                  </div>
                </div>
              );

              // PAYG fallback info
              if (qd.subscription.payg_fallback_enabled) {
                const spent = qd.subscription.payg_spent_usd;
                const limit = qd.subscription.payg_limit_usd;
                const fallbackPct = spent != null && limit != null && limit > 0 ? (spent / limit) * 100 : 0;
                items.push(
                  <div key='fallback' className='border-border/60 space-y-2.5 border-t border-dashed pt-3'>
                    <div className='space-y-1'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='text-muted-foreground font-medium'>
                          {t('quota.label.payg_fallback')}
                          {spent != null && limit != null && (
                            <span className='font-normal opacity-70'>
                              {' '}
                              (${spent.toFixed(2)}/${limit.toFixed(2)})
                            </span>
                          )}
                        </span>
                        <span className='text-foreground font-medium'>
                          {spent != null && limit != null ? t('quota.label.percent_used', { percent: Math.round(fallbackPct) }) : ''}
                        </span>
                      </div>
                      {spent != null && limit != null && limit > 0 && <ProgressBar percentage={fallbackPct} />}
                    </div>
                  </div>
                );
              }
            }

            // For active subscribers without PAYG fallback, PAYG is only meaningful if there are real credits.
            // Otherwise it's just noise (0 credits, unlimited tokens, no fallback = nothing to show).
            const hasPaygCredits =
              qd.payg && (qd.payg.account_credits > 0 || (typeof qd.payg.token_used === 'number' && qd.payg.token_used > 0));
            const isPaygRelevant =
              !qd.is_subscriber || qd.subscription?.status !== 'active' || qd.subscription?.payg_fallback_enabled || hasPaygCredits;
            if (
              isPaygRelevant &&
              qd.payg &&
              !qd.payg.token_is_unlimited &&
              typeof qd.payg.token_total === 'number' &&
              typeof qd.payg.token_used === 'number' &&
              qd.payg.token_total > 0
            ) {
              const tokenUsed = qd.payg.token_used;
              const tokenTotal = qd.payg.token_total;
              const tokenPct = (tokenUsed / tokenTotal) * 100;
              const hasSubSection = items.length > 0;
              items.push(
                <div key='payg' className={hasSubSection ? 'border-border/60 space-y-2.5 border-t border-dashed pt-3' : 'space-y-2.5'}>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t('quota.label.token_usage')}
                        <span className='font-normal opacity-70'>
                          {' '}
                          (${tokenUsed.toFixed(2)}/${tokenTotal.toFixed(2)})
                        </span>
                      </span>
                      <span className='text-foreground font-medium'>
                        {t('quota.label.percent_used', { percent: Math.round(tokenPct) })}
                      </span>
                    </div>
                    <ProgressBar percentage={tokenPct} />
                  </div>
                </div>
              );
            }

            // Account balance — hidden for active subscribers without meaningful PAYG
            if (isPaygRelevant && qd.payg && qd.payg.account_credits !== undefined) {
              const hasSubSection = items.length > 0;
              items.push(
                <div key='balance' className={hasSubSection ? 'border-border/60 space-y-2.5 border-t border-dashed pt-3' : 'space-y-2.5'}>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='text-muted-foreground font-medium'>{t('quota.label.account_balance')}</span>
                    <span className='text-foreground font-medium'>
                      {qd.payg.token_is_unlimited
                        ? `${t('quota.label.unlimited')} · $${qd.payg.account_credits.toFixed(2)}`
                        : `$${qd.payg.account_credits.toFixed(2)}`}
                    </span>
                  </div>
                </div>
              );
            }

            // Monthly token spending limit (if configured) — hidden for active subscribers without fallback
            if (isPaygRelevant && qd.payg?.token_monthly_limit_usd != null && qd.payg.token_monthly_used_usd != null) {
              const monthlyPct =
                qd.payg.token_monthly_limit_usd > 0 ? (qd.payg.token_monthly_used_usd / qd.payg.token_monthly_limit_usd) * 100 : 0;
              items.push(
                <div key='monthly' className='border-border/60 space-y-2.5 border-t border-dashed pt-3'>
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>
                        {t('quota.label.monthly_limit')}
                        <span className='font-normal opacity-70'>
                          {' '}
                          (${qd.payg.token_monthly_used_usd.toFixed(2)}/${qd.payg.token_monthly_limit_usd.toFixed(2)})
                        </span>
                      </span>
                      <span className='text-foreground font-medium'>
                        {t('quota.label.percent_used', { percent: Math.round(monthlyPct) })}
                      </span>
                    </div>
                    <ProgressBar percentage={monthlyPct} />
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

      {isOpenaiType(channel.type) && channel.providerType === 'charm_hyper' && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderCharmHyperQuotaData | undefined;
            if (!qd || typeof qd.balance !== 'number') return null;
            const balance = qd.balance;
            const usedPct = Math.max(0, Math.min(100, (1.0 - balance / 100) * 100));
            return (
              <div className='space-y-2.5'>
                <div className='space-y-1'>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='text-muted-foreground font-medium'>{t('quota.label.credits_remaining')}</span>
                    <span className='text-foreground font-medium'>{Number.isInteger(balance) ? balance : balance.toFixed(2)}</span>
                  </div>
                  <ProgressBar percentage={usedPct} />
                </div>
              </div>
            );
          })()}
        </div>
      )}

      {isZenmuxType(channel.type) && (
        <div className='mt-3 space-y-3'>
          {(() => {
            const qd = channel.quotaStatus.quotaData as ProviderZenmuxQuotaData | undefined;
            if (!qd) return null;
            const items: React.ReactNode[] = [];

            // Channels configured with the same ZenMux management key share one
            // account, so the popover shows one row per account and lists the
            // member channels behind it.
            const sharedNames = channel.sharedAccountNames ?? [];
            if (sharedNames.length > 1) {
              items.push(
                <div key='shared-account' className='bg-muted/40 text-muted-foreground rounded p-2 text-[11px]'>
                  <div>{t('quota.label.shared_account_channels', { count: sharedNames.length })}</div>
                  <div>{sharedNames.join(', ')}</div>
                </div>
              );
            }

            const monthlyUSD = getZenmuxMonthlyQuotaUSD(qd);
            if (qd.plan?.tier || monthlyUSD != null) {
              const tier = qd.plan?.tier ? capitalizeZenmuxTier(qd.plan.tier) : '';
              const monthlyQuota =
                monthlyUSD != null
                  ? ` (${t('currencies.format', { val: monthlyUSD, currency: 'USD', locale: 'en-US', minimumFractionDigits: 2, maximumFractionDigits: 2 })})`
                  : '';
              items.push(
                <div key='plan' className='flex items-center justify-between text-xs'>
                  <span className='text-muted-foreground font-medium'>{t('quota.label.plan')}</span>
                  <span className='text-foreground font-medium'>
                    {tier}
                    {monthlyQuota}
                  </span>
                </div>
              );
            }

            // 5h/7d windows reuse the shared window progress-bar rendering driven
            // by the normalized _limits data (window/usageRatio/nextResetAt).
            quota.limits
              .filter((limit) => limit.window === '5h' || limit.window === '7d')
              .sort((a, b) => (a.window === '5h' ? -1 : b.window === '5h' ? 1 : 0))
              .forEach((limit) => {
                const labelKey = limit.window ? WINDOW_LABEL_KEYS[limit.window] : undefined;
                if (!labelKey) return;
                const usedPct = limit.usageRatio * 100;
                const durationPct = getLimitDurationPercent(limit);
                const resetText = limit.nextResetAt ? formatTimeToReset(limit.nextResetAt) : '';

                items.push(
                  <div
                    key={limit.window}
                    className={items.length > 0 ? 'border-border/60 space-y-1.5 border-t border-dashed pt-3' : 'space-y-1.5'}
                  >
                    <div className='flex items-center justify-between text-xs'>
                      <span className='text-muted-foreground font-medium'>{t(labelKey)}</span>
                      <span className='text-foreground font-medium'>
                        {t('quota.label.percent_used', { percent: Math.round(usedPct) })}
                      </span>
                    </div>
                    <UsageTimeBar
                      usagePercent={usedPct}
                      durationPercent={durationPct}
                      tooltip={
                        <div className='space-y-0.5'>
                          <div className='font-medium'>{t(labelKey)}</div>
                          <div>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</div>
                          {durationPct !== undefined && (
                            <div>
                              {t('quota.label.time_elapsed')}: {Math.round(durationPct)}%
                            </div>
                          )}
                          {resetText && <div>{resetText}</div>}
                        </div>
                      }
                    />
                  </div>
                );
              });


            if (items.length === 0) {
              items.push(
                <div key='unavailable' className='bg-muted/40 text-muted-foreground rounded p-2 text-[11px]'>
                  {t('quota.label.unavailable')}
                </div>
              );
            }

            return items;
          })()}
        </div>
      )}

      <PeriodQuotaEstimate limits={quota.limits} />
    </div>
  );
}

function QuotaBadgeTrigger({ channels, isLoading, isError }: { channels: ProviderQuotaChannel[]; isLoading?: boolean; isError?: boolean }) {
  if (isLoading) {
    return <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />;
  }

  if (isError) {
    return <BatteryWarning className='h-5 w-5 text-red-500 transition-colors' />;
  }

  const highestUsed = Math.max(...channels.map(getChannelPercentage));

  const hasExhausted = channels.some((c) => c.quotaStatus.status === 'exhausted');
  const hasWarning = channels.some((c) => c.quotaStatus.status === 'warning');

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
  const { channels, isLoading, isError, error } = useProviderQuotaStatuses();
  const { data: enforcementSettings } = useQuotaEnforcementSettings();
  const enforcementMode = enforcementSettings?.enabled ? enforcementSettings.mode : null;
  const allowedChannelIDs = enforcementSettings?.enabled ? enforcementSettings.allowedChannelIDs : null;

  if (!isLoading && !isError && channels.length === 0) return null;

  const groupedChannels = channels.reduce((acc: ProviderQuotaChannel[], channel: ProviderQuotaChannel) => {
    if (channel.type === 'nanogpt_responses') {
      const existing = acc.find((c) => c.type === 'nanogpt');
      if (!existing) {
        acc.push(channel);
      }
    } else if (isOpenaiType(channel.type) && channel.providerType) {
      const existing = acc.find((c) => isOpenaiType(c.type) && c.providerType === channel.providerType);
      if (!existing) {
        acc.push(channel);
      }
    } else if (channel.accountKey) {
      // Channels sharing one ZenMux account (same non-empty accountKey) collapse
      // into a single row: the first channel carries the shared quota, later
      // members only contribute their names to the shared-account note. The
      // representative is a shallow copy so the parsed query data stays untouched.
      const existing = acc.find((c) => c.accountKey === channel.accountKey);
      if (existing) {
        existing.sharedAccountNames = [...(existing.sharedAccountNames ?? [existing.name]), channel.name];
      } else {
        acc.push({ ...channel, sharedAccountNames: [channel.name] });
      }
    } else {
      acc.push(channel);
    }
    return acc;
  }, [] as ProviderQuotaChannel[]);

  const renderContent = () => {
    if (isLoading) {
      return (
        <div className='text-muted-foreground flex items-center gap-2 px-1 py-3 text-sm'>
          <Loader2 className='h-4 w-4 animate-spin' />
          <span>{t('system.providerQuota.loading')}</span>
        </div>
      );
    }

    if (isError) {
      return (
        <div className='rounded bg-red-500/10 p-2 text-xs break-words text-red-500'>
          <span className='font-medium'>{t('system.providerQuota.error')}:</span> {error instanceof Error ? error.message : t('quota.label.unavailable')}
        </div>
      );
    }

    return (
      <div
        className={`max-h-[60vh] overflow-y-auto pr-1 pl-1 ${groupedChannels.length > 4 ? 'grid grid-cols-1 gap-x-4 sm:grid-cols-2' : ''}`}
      >
        {groupedChannels.map((channel: ProviderQuotaChannel) => (
          <QuotaRow key={channel.id} channel={channel} enforcementMode={enforcementMode} allowedChannelIDs={allowedChannelIDs} />
        ))}
      </div>
    );
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button type='button' className='hover:bg-muted relative rounded-md p-2 transition-colors'>
          <QuotaBadgeTrigger channels={groupedChannels} isLoading={isLoading} isError={isError} />
        </button>
      </PopoverTrigger>
      <PopoverContent
        className={
          !isLoading && !isError && groupedChannels.length > 4
            ? 'w-[640px] max-w-[calc(100vw-2rem)]'
            : 'w-80 max-w-[calc(100vw-2rem)]'
        }
        align='end'
      >
        <div className='space-y-1'>
          <div className='mb-2 flex items-center justify-between'>
            <div className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>{t('system.providerQuota.title')}</div>
            <button
              onClick={onRefresh}
              disabled={isRefreshing || isLoading}
              className='text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50'
              aria-label={t('system.providerQuota.refresh.label')}
            >
              {isRefreshing ? <Loader2 className='h-4 w-4 animate-spin' /> : <RefreshCw className='h-4 w-4' />}
            </button>
          </div>
          {renderContent()}
        </div>
      </PopoverContent>
    </Popover>
  );
}
