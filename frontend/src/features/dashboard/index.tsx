import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from '@tanstack/react-router';
import { ChevronRight, TrendingUp } from 'lucide-react';
import { Main } from '@/components/layout/main';
import { TimeRangeFilter } from '@/components/time-range-filter';
import { DEFAULT_TIME_RANGE, useDashboardTimeStore } from '@/stores/dashboardStore';
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
import { todayLocalDate } from './utils/time-window';
import { formatNumber } from '@/utils/format-number';

/** Dashboard page: a fixed pulse strip first, then the analysis range drives
 * everything below it — trend, performance, distribution and composition. */
export default function DashboardPage() {
  const { t } = useTranslation();
  const { startTime, endTime, timeWindow, setRange } = useDashboardTimeStore();
  const { data: generalSettings } = useGeneralSettings();
  const { data: metadata } = useAnalyticsMetadata();
  const { isProjectOwner } = useRoutePermissions();

  const filter = useMemo<AnalyticsFilter>(() => ({ startTime, endTime, timeWindow }), [startTime, endTime, timeWindow]);

  const { data: overview, isLoading: isOverviewLoading, error: overviewError } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading, error: dailyError } = useAnalyticsDailyStats(filter);

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const loadError = overviewError || dailyError;

  // The trend query applies no time filter at all when it gets neither dates nor a
  // relative window, so "since first use" is what the badge has to say in that case.
  const rangeBadgeLabel = startTime ? `${startTime} – ${endTime ?? todayLocalDate()}` : t(`timeRange.${timeWindow ?? 'allTime'}`);

  const rangeSummary = [
    { key: 'requests', label: t('analytics.overview.totalRequests'), value: formatNumber(overview?.totalRequests) },
    { key: 'tokens', label: t('analytics.overview.totalTokens'), value: formatNumber(overview?.totalTokens) },
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
            value={{ startTime, endTime, timeWindow }}
            onChange={setRange}
            earliestDate={metadata?.earliestDate}
            defaultValue={DEFAULT_TIME_RANGE}
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
                    badge={<RangeBadge label={rangeBadgeLabel} />}
                  />
                </div>
                <div className='col-span-1 lg:col-span-3'>
                  <ChannelHealthCard startTime={startTime} endTime={endTime} timeWindow={timeWindow} />
                </div>
              </div>

              <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-7'>
                <div className='col-span-1 lg:col-span-4'>
                  <PerformanceCard startTime={startTime} endTime={endTime} timeWindow={timeWindow} />
                </div>
                <div className='col-span-1 lg:col-span-3'>
                  <FastestPerformersCard startTime={startTime} endTime={endTime} timeWindow={timeWindow} />
                </div>
              </div>

              <RequestCostDistribution startTime={startTime} endTime={endTime} timeWindow={timeWindow} currencyCode={currencyCode} />

              <TokenComposition
                startTime={startTime}
                endTime={endTime}
                timeWindow={timeWindow}
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
