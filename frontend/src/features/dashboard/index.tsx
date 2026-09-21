import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from '@tanstack/react-router';
import { ChevronRight, Globe, TrendingUp } from 'lucide-react';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Skeleton } from '@/components/ui/skeleton';
import { TimeRangeFilter } from '@/components/time-range-filter';
import { useDashboardTimeStore } from '@/stores/dashboardStore';
import { useGeneralSettings } from '@/features/system/data/system';
import {
  useAnalyticsDailyStats,
  useAnalyticsMetadata,
  useAnalyticsOverview,
  type AnalyticsFilter,
} from '@/features/analytics/data/analytics';
import { CombinedTrendChart } from '@/features/analytics/components/combined-trend-chart';
import { ChannelHealthCard } from './components/channel-health-card';
import { FastestChannelsCard } from './components/fastest-channels-card';
import { FastestModelsCard } from './components/fastest-models-card';
import { KpiRow } from './components/kpi-row';
import type { CoarseTimeWindow } from './utils/time-window';

function toCoarseWindow(startTime: string | null, endTime: string | null): CoarseTimeWindow {
  if (!startTime) return 'month';

  const end = endTime ? new Date(endTime) : new Date();
  const days = Math.round((end.getTime() - new Date(startTime).getTime()) / 86_400_000) + 1;

  if (days <= 1) return 'day';
  if (days <= 7) return 'week';
  return 'month';
}

export default function DashboardPage() {
  const { t } = useTranslation();
  const { startTime, endTime, setRange } = useDashboardTimeStore();
  const { data: generalSettings } = useGeneralSettings();
  const { data: metadata } = useAnalyticsMetadata();

  const filter = useMemo<AnalyticsFilter>(() => ({ startTime, endTime }), [startTime, endTime]);

  const { data: overview, isLoading: isOverviewLoading, error: overviewError } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading, error: dailyError } = useAnalyticsDailyStats(filter);

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const performanceWindow = toCoarseWindow(startTime, endTime);
  const loadError = overviewError || dailyError;

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('sidebar.items.dashboard')}</h2>
            <p className='text-sm text-muted-foreground'>{t('dashboard.description')}</p>
          </div>
          <span className='text-muted-foreground flex items-center gap-1.5 text-xs'>
            <Globe className='h-3.5 w-3.5' />
            {t('dashboard.scope.system')}
          </span>
        </div>
      </Header>

      <Main fixed>
        <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-auto'>
          <TimeRangeFilter
            value={{ startTime, endTime }}
            onChange={setRange}
            earliestDate={metadata?.earliestDate}
          />

          {loadError ? (
            <div className='text-sm text-red-500'>
              {t('common.loadError')} {loadError.message}
            </div>
          ) : (
            <>
              <KpiRow overview={overview} isLoading={isOverviewLoading} />

              <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
                <div className='col-span-1 lg:col-span-4'>
                  {isDailyLoading && !dailyStats ? (
                    <Skeleton className='h-[410px] w-full' />
                  ) : (
                    <CombinedTrendChart data={dailyStats || []} isLoading={isDailyLoading} currencyCode={currencyCode} />
                  )}
                </div>
                <div className='col-span-1 lg:col-span-3'>
                  <ChannelHealthCard startTime={startTime} endTime={endTime} />
                </div>
              </div>

              <div className='grid gap-4 md:grid-cols-2'>
                <FastestChannelsCard timeWindow={performanceWindow} />
                <FastestModelsCard timeWindow={performanceWindow} />
              </div>
            </>
          )}

          <Link
            to='/analytics'
            className='flex w-full shrink-0 items-center justify-between rounded-lg border bg-card p-4 text-left transition-colors hover:bg-accent/50'
          >
            <div className='flex items-center gap-3'>
              <div className='flex h-8 w-8 items-center justify-center rounded-md bg-primary/10'>
                <TrendingUp className='h-4 w-4 text-primary' />
              </div>
              <div>
                <span className='text-lg font-semibold'>{t('analytics.title')}</span>
                <p className='text-sm text-muted-foreground'>{t('dashboard.sections.analyticsDescription')}</p>
              </div>
            </div>
            <ChevronRight className='h-5 w-5 text-muted-foreground' />
          </Link>
        </div>
      </Main>
    </div>
  );
}
