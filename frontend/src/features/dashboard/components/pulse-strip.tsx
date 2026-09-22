import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { IconInfoCircle } from '@tabler/icons-react';
import { Activity, BarChart4, Database, ShieldCheck } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatDuration } from '@/utils/format-duration';
import { formatNumber } from '@/utils/format-number';
import { useDashboardStats, useTokenStats, type TokenStats } from '../data/dashboard';

type TokenRange = 'allTime' | 'thisMonth' | 'thisWeek' | 'thisDay';

const TOKEN_RANGES: TokenRange[] = ['allTime', 'thisMonth', 'thisWeek', 'thisDay'];

/** Composition colors, matching the trend chart legend so both read as one system. */
const COLOR_INPUT = 'var(--chart-2)';
const COLOR_CACHED = 'var(--chart-1)';
const COLOR_OUTPUT = 'var(--chart-3)';

interface PulseCardShellProps {
  title: string;
  icon: React.ReactNode;
  action?: React.ReactNode;
  accent?: boolean;
  children: React.ReactNode;
}

function PulseCardShell({ title, icon, action, accent, children }: PulseCardShellProps) {
  return (
    <Card className={`hover-card min-w-0 ${accent ? 'bg-primary text-primary-foreground' : ''}`}>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex min-w-0 items-center gap-2'>
          {icon}
          <CardTitle className={`truncate text-sm font-medium ${accent ? 'text-primary-foreground/90' : ''}`}>{title}</CardTitle>
        </div>
        {action}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

function PulseSkeleton() {
  return (
    <div className='space-y-2'>
      <Skeleton className='h-9 w-[110px]' />
      <Skeleton className='h-3 w-[150px]' />
    </div>
  );
}

function TokensLastUpdatedInfo({ lastUpdated }: { lastUpdated: string | null }) {
  const { t, i18n } = useTranslation();

  if (!lastUpdated) return null;

  const label = t('dashboard.stats.updated', {
    time: new Date(lastUpdated).toLocaleString(i18n.language, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    }),
  });

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type='button'
          aria-label={label}
          className='text-muted-foreground hover:text-foreground flex h-5 w-5 items-center justify-center rounded-full transition-colors'
        >
          <IconInfoCircle className='h-3.5 w-3.5' />
        </button>
      </PopoverTrigger>
      <PopoverContent className='w-fit'>
        <span className='text-sm'>{label}</span>
      </PopoverContent>
    </Popover>
  );
}

function tokensForRange(stats: TokenStats | undefined, range: TokenRange) {
  if (range === 'allTime') {
    return {
      input: stats?.totalInputTokensAllTime || 0,
      output: stats?.totalOutputTokensAllTime || 0,
      cached: stats?.totalCachedTokensAllTime || 0,
    };
  }
  if (range === 'thisDay') {
    return {
      input: stats?.totalInputTokensToday || 0,
      output: stats?.totalOutputTokensToday || 0,
      cached: stats?.totalCachedTokensToday || 0,
    };
  }
  if (range === 'thisMonth') {
    return {
      input: stats?.totalInputTokensThisMonth || 0,
      output: stats?.totalOutputTokensThisMonth || 0,
      cached: stats?.totalCachedTokensThisMonth || 0,
    };
  }
  return {
    input: stats?.totalInputTokensThisWeek || 0,
    output: stats?.totalOutputTokensThisWeek || 0,
    cached: stats?.totalCachedTokensThisWeek || 0,
  };
}

function TodayRequestsPulseCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading } = useDashboardStats();

  return (
    <PulseCardShell
      accent
      title={t('dashboard.stats.todayRequests')}
      icon={<Activity className='text-primary-foreground/70 h-4 w-4' />}
      action={<div className='bg-primary-foreground h-2 w-2 animate-ping rounded-full' />}
    >
      {isLoading ? (
        <PulseSkeleton />
      ) : (
        <div className='space-y-4'>
          <div className='font-mono text-4xl font-bold tracking-tight'>{formatNumber(stats?.requestStats?.requestsToday || 0)}</div>
          <div className='border-primary-foreground/10 text-primary-foreground/70 flex justify-between border-t pt-3 text-xs'>
            <span>
              {t('dashboard.stats.thisWeek')}: {formatNumber(stats?.requestStats?.requestsThisWeek || 0)}
            </span>
            <span>
              {t('dashboard.stats.thisMonth')}: {formatNumber(stats?.requestStats?.requestsThisMonth || 0)}
            </span>
          </div>
        </div>
      )}
    </PulseCardShell>
  );
}

function AllTimeRequestsPulseCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading } = useDashboardStats();

  const current = stats?.requestStats?.requestsThisWeek || 0;
  const previous = stats?.requestStats?.requestsLastWeek || 0;
  const growth = previous === 0 ? (current > 0 ? 100 : 0) : ((current - previous) / previous) * 100;
  const isPositive = growth >= 0;

  return (
    <PulseCardShell
      title={t('dashboard.stats.allTimeRequests')}
      icon={
        <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
          <Database className='h-4 w-4' />
        </div>
      }
    >
      {isLoading ? (
        <PulseSkeleton />
      ) : (
        <div className='space-y-2'>
          <div className='font-mono text-3xl font-bold'>{formatNumber(stats?.totalRequests || 0)}</div>
          <div className={`flex items-center gap-1.5 text-xs font-medium ${isPositive ? 'text-primary' : 'text-red-500'}`}>
            <span
              className={`rounded-md px-1.5 py-0.5 ${
                isPositive ? 'border-primary/20 bg-primary/10 border' : 'border border-red-500/20 bg-red-500/10'
              }`}
            >
              {isPositive ? '+' : ''}
              {growth.toFixed(0)}%
            </span>
            <span className='text-muted-foreground'>{t('dashboard.stats.vsLastWeek')}</span>
          </div>
        </div>
      )}
    </PulseCardShell>
  );
}

function TokenStatsPulseCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading } = useTokenStats();
  const [range, setRange] = useState<TokenRange>('allTime');

  const tokens = tokensForRange(stats, range);
  const total = tokens.input + tokens.output;
  const uncached = Math.max(tokens.input - tokens.cached, 0);
  const parts = [
    { key: 'input', value: uncached, color: COLOR_INPUT, label: t('dashboard.stats.input') },
    { key: 'cached', value: tokens.cached, color: COLOR_CACHED, label: t('dashboard.stats.cached') },
    { key: 'output', value: tokens.output, color: COLOR_OUTPUT, label: t('dashboard.stats.output') },
  ];

  return (
    <PulseCardShell
      title={t('dashboard.cards.tokenStats')}
      icon={
        <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
          <BarChart4 className='h-4 w-4' />
        </div>
      }
      action={
        <div className='flex shrink-0 items-center gap-1'>
          {range === 'allTime' && <TokensLastUpdatedInfo lastUpdated={stats?.lastUpdated ?? null} />}
          <Tabs value={range} onValueChange={(value) => setRange(value as TokenRange)}>
            <TabsList className='h-6 p-0.5'>
              {TOKEN_RANGES.map((value) => (
                <TabsTrigger key={value} value={value} className='h-5 px-1.5 text-[10px]'>
                  {t(`dashboard.stats.${value === 'allTime' ? 'all' : value === 'thisMonth' ? 'month' : value === 'thisWeek' ? 'week' : 'day'}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      }
    >
      {isLoading ? (
        <PulseSkeleton />
      ) : (
        <div className='space-y-3'>
          <div className='font-mono text-3xl font-bold'>{formatNumber(total)}</div>
          <div className='flex h-2 overflow-hidden rounded-full bg-muted'>
            {parts.map((part) => (
              <div
                key={part.key}
                style={{ width: `${total > 0 ? (part.value / total) * 100 : 0}%`, backgroundColor: part.color }}
              />
            ))}
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
            {parts.map((part) => (
              <span key={part.key} className='flex items-center gap-1'>
                <span className='h-2 w-2 rounded-full' style={{ backgroundColor: part.color }} />
                {part.label} {formatNumber(part.value)}
              </span>
            ))}
          </div>
        </div>
      )}
    </PulseCardShell>
  );
}

function SuccessRatePulseCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading } = useDashboardStats();

  const total = stats?.totalRequests || 0;
  const failed = stats?.failedRequests || 0;
  const succeeded = Math.max(total - failed, 0);
  const successRate = total > 0 ? (succeeded / total) * 100 : 0;

  const parts = [
    { key: 'succeeded', value: succeeded, color: 'var(--primary)', label: t('dashboard.stats.succeeded') },
    { key: 'failed', value: failed, color: 'var(--destructive)', label: t('dashboard.stats.failedRequests') },
  ];

  return (
    <PulseCardShell
      title={t('dashboard.cards.successRate')}
      icon={
        <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
          <ShieldCheck className='h-4 w-4' />
        </div>
      }
    >
      {isLoading ? (
        <PulseSkeleton />
      ) : (
        <div className='space-y-3'>
          <div className='font-mono text-3xl font-bold'>
            {successRate.toFixed(1)}
            <span className='text-muted-foreground ml-1 text-lg font-semibold'>%</span>
          </div>
          <div className='flex h-2 overflow-hidden rounded-full bg-muted'>
            {parts.map((part) => (
              <div
                key={part.key}
                style={{ width: `${total > 0 ? (part.value / total) * 100 : 0}%`, backgroundColor: part.color }}
              />
            ))}
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
            {parts.map((part) => (
              <span key={part.key} className='flex items-center gap-1'>
                <span className='h-2 w-2 rounded-full' style={{ backgroundColor: part.color }} />
                {part.label} {formatNumber(part.value)}
              </span>
            ))}
            {stats?.averageResponseTime != null && (
              <span className='flex items-center gap-1'>
                {t('dashboard.stats.average')} {formatDuration(stats.averageResponseTime)}
              </span>
            )}
          </div>
        </div>
      )}
    </PulseCardShell>
  );
}

/** Fixed pulse strip: today, all time, token stats and success rate. These figures
 * answer "how am I doing right now", so they are deliberately independent of the
 * analysis range below. */
export function PulseStrip() {
  return (
    <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-4'>
      <TodayRequestsPulseCard />
      <AllTimeRequestsPulseCard />
      <TokenStatsPulseCard />
      <SuccessRatePulseCard />
    </div>
  );
}
