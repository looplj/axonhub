import { useTranslation } from 'react-i18next';
import { Activity, Gauge } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatDuration } from '@/utils/format-duration';
import { formatNumber } from '@/utils/format-number';
import { useDashboardStats } from '../data/dashboard';
import { SuccessRateCard } from './success-rate-card';
import { TokenStatsCard } from './token-stats-card';

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

function Last24HoursPerformancePulseCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading } = useDashboardStats();

  const performance = stats?.last24HoursPerformance;
  const throughput = performance?.throughput ?? null;
  // Both percentiles are read off the same streaming sample, so a null on either side
  // means the window holds no completed streaming generation. Rendering "0ms" instead
  // would read as "measured, and instant".
  const latency =
    performance?.firstTokenP90Ms != null && performance.firstTokenP50Ms != null
      ? { p90: performance.firstTokenP90Ms, p50: performance.firstTokenP50Ms }
      : null;

  return (
    <PulseCardShell
      title={t('dashboard.stats.last24hPerformance')}
      icon={
        <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
          <Gauge className='h-4 w-4' />
        </div>
      }
    >
      {isLoading ? (
        <PulseSkeleton />
      ) : (
        <div className='space-y-2'>
          <div className='font-mono text-3xl font-bold'>
            {throughput != null ? (
              <>
                {formatNumber(throughput, { digits: 0 })}
                <span className='text-muted-foreground ml-1 text-lg font-semibold'>{t('dashboard.stats.throughput')}</span>
              </>
            ) : (
              <span className='text-muted-foreground'>&mdash;</span>
            )}
          </div>
          {latency && (
            <div className='text-muted-foreground flex flex-wrap items-baseline gap-x-1.5 gap-y-1 text-xs'>
              <span>TTFT</span>
              <span className='text-foreground font-mono tabular-nums'>{formatDuration(latency.p90)}</span>
              <span className='text-primary font-medium'>(p90)</span>
              <span className='text-foreground font-mono tabular-nums'>{formatDuration(latency.p50)}</span>
              <span className='text-primary font-medium'>(p50)</span>
            </div>
          )}
        </div>
      )}
    </PulseCardShell>
  );
}

/** Fixed pulse strip: requests today, trailing-24h performance, the token statistics
 * card and the trailing-24h success rate. These figures answer "how am I doing right
 * now", so they are deliberately independent of the analysis range below. The token
 * and success-rate cards keep the long-standing standalone card design. */
export function PulseStrip() {
  return (
    <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-4'>
      <TodayRequestsPulseCard />
      <Last24HoursPerformancePulseCard />
      <TokenStatsCard />
      <SuccessRateCard />
    </div>
  );
}
