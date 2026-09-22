import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from '@tanstack/react-router';
import { ChevronRight, TrendingUp } from 'lucide-react';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Skeleton } from '@/components/ui/skeleton';
import { TimeRangeFilter } from '@/components/time-range-filter';
import { useDashboardTimeStore } from '@/stores/dashboardStore';
import { useGeneralSettings } from '@/features/system/data/system';
import { useAnalyticsDailyStats, useAnalyticsMetadata, useAnalyticsOverview, type AnalyticsFilter } from '@/features/analytics/data/analytics';
import { CombinedTrendChart } from '@/features/analytics/components/combined-trend-chart';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { ChannelHealthCard } from './components/channel-health-card';
import { FastestPerformersCard } from './components/fastest-performers-card';
import { PerformanceCard } from './components/performance-card';
import { PulseStrip } from './components/pulse-strip';
import { RangeBadge } from './components/range-badge';
import { RequestCostDistribution } from './components/request-cost-distribution';
import { TokenComposition } from './components/token-composition';

/** Dashboard page: a fixed pulse strip first, then the analysis range drives
 * everything below it — trend, performance, distribution and composition. */
export default function DashboardPage() {
  const { t } = useTranslation();
  const { startTime, endTime, setRange } = useDashboardTimeStore();
  const { data: generalSettings } = useGeneralSettings();
  const { data: metadata } = useAnalyticsMetadata();
  const { isProjectOwner } = useRoutePermissions();

  const filter = useMemo<AnalyticsFilter>(() => ({ startTime, endTime }), [startTime, endTime]);

  const { data: overview, isLoading: isOverviewLoading, error: overviewError } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading, error: dailyError } = useAnalyticsDailyStats(filter);

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const loadError = overviewError || dailyError;

  const rangeSummary = [
    { key: 'requests', label: t('analytics.overview.totalRequests'), value: Math.round(overview?.totalRequests || 0).toLocaleString() },
    { key: 'tokens', label: t('analytics.overview.totalTokens'), value: Math.round(overview?.totalTokens || 0).toLocaleString() },
    {
      key: 'cost',
      label: t('analytics.overview.totalCost'),
      value: t('currencies.format', {
        val: overview?.totalCost || 0,
        currency: currencyCode,
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }),
    },
  ];

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <h2 className='text-xl font-bold tracking-tight'>{t('sidebar.items.dashboard')}</h2>
      </Header>

      <Main fixed>
        <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-auto'>
          <PulseStrip />

          <TimeRangeFilter
            variant='compact'
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
              <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
                <div className='col-span-1 lg:col-span-4'>
                  {isDailyLoading && !dailyStats ? (
                    <Skeleton className='h-[410px] w-full' />
                  ) : (
                    <CombinedTrendChart
                      data={dailyStats || []}
                      isLoading={isDailyLoading}
                      currencyCode={currencyCode}
                      badge={<RangeBadge label={startTime ? `${startTime} – ${endTime || startTime}` : t('timeRange.last30Days')} />}
                      summary={
                        isOverviewLoading
                          ? rangeSummary.map((item) => <Skeleton key={item.key} className='h-3 w-24' />)
                          : rangeSummary.map((item) => (
                              <span key={item.key}>
                                {item.label}: <span className='text-foreground font-mono font-medium tabular-nums'>{item.value}</span>
                              </span>
                            ))
                      }
                    />
                  )}
                </div>
                <div className='col-span-1 lg:col-span-3'>
                  <ChannelHealthCard startTime={startTime} endTime={endTime} />
                </div>
              </div>

              <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-7'>
                <div className='col-span-1 lg:col-span-4'>
                  <PerformanceCard />
                </div>
                <div className='col-span-1 lg:col-span-3'>
                  <FastestPerformersCard startTime={startTime} endTime={endTime} />
                </div>
              </div>

              <RequestCostDistribution startTime={startTime} endTime={endTime} currencyCode={currencyCode} />

              <TokenComposition
                startTime={startTime}
                endTime={endTime}
                currencyCode={currencyCode}
                isProjectOwner={isProjectOwner}
              />
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
