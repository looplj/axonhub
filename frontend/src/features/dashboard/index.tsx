import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from '@tanstack/react-router';
import { ChevronRight, TrendingUp } from 'lucide-react';
import { Main } from '@/components/layout/main';
import { TimeRangeFilter } from '@/components/time-range-filter';
import { useDashboardTimeStore } from '@/stores/dashboardStore';
import { useGeneralSettings } from '@/features/system/data/system';
import { useAnalyticsDailyStats, useAnalyticsMetadata, useAnalyticsOverview, type AnalyticsFilter } from '@/features/analytics/data/analytics';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { ChannelHealthCard } from './components/channel-health-card';
import { DailyOverviewChart } from './components/daily-overview-chart';
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
      <Main fixed className='px-0 py-0'>
        <div className='flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-8 pt-6 pb-8'>
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
              <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-7'>
                <div className='col-span-1 lg:col-span-4'>
                  <DailyOverviewChart
                    data={dailyStats || []}
                    isLoading={isDailyLoading}
                    currencyCode={currencyCode}
                    isOverviewLoading={isOverviewLoading}
                    rangeSummary={rangeSummary}
                    badge={<RangeBadge label={startTime ? `${startTime} – ${endTime || startTime}` : t('timeRange.last30Days')} />}
                  />
                </div>
                <div className='col-span-1 lg:col-span-3'>
                  <ChannelHealthCard startTime={startTime} endTime={endTime} />
                </div>
              </div>

              <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-7'>
                <div className='col-span-1 lg:col-span-4'>
                  <PerformanceCard startTime={startTime} endTime={endTime} />
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
