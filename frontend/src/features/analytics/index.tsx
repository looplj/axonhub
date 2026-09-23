import { useMemo, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Main } from '@/components/layout/main';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { useDashboardTimeStore } from '@/stores/dashboardStore';
import { useAnalyticsMetadata, useAnalyticsOverview, useAnalyticsDailyStats, useAnalyticsDimensionStats, type AnalyticsFilter } from './data/analytics';
import { AnalyticsFilterBar } from './components/analytics-filter-bar';
import { OverviewCards } from './components/overview-cards';
import { UsageCompositionChart } from './components/usage-composition-chart';
import { DimensionDistribution, type DistributionMetric } from './components/dimension-distribution';
import { DimensionShareStrip } from './components/dimension-share-strip';
import { DimensionDetailTable } from './components/dimension-detail-table';
import { useGeneralSettings } from '@/features/system/data/system';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';

const METRIC_STORAGE_KEY = 'analytics-distribution-metric';
const METRICS: DistributionMetric[] = ['requestCount', 'totalTokens', 'cost'];

/** Cost is the default: it carries the most decision value, so the page opens on the
 * question "where is my money concentrated" rather than on raw volume. */
function readStoredMetric(): DistributionMetric {
  try {
    const stored = localStorage.getItem(METRIC_STORAGE_KEY);
    return stored === 'requestCount' || stored === 'totalTokens' || stored === 'cost' ? stored : 'cost';
  } catch {
    return 'cost';
  }
}

/** Analytics page: overview cards, trend chart and per-dimension breakdowns, all
 * driven by the shared filter bar. */
export default function AnalyticsPage() {
  const { t } = useTranslation();
  const dimensions = useAnalyticsFilterStore((state) => state.dimensions);
  const { startTime, endTime } = useDashboardTimeStore();
  const { data: generalSettings } = useGeneralSettings();
  const { isProjectOwner } = useRoutePermissions();
  const [metric, setMetric] = useState<DistributionMetric>(readStoredMetric);

  const currencyCode = generalSettings?.currencyCode || 'USD';

  const filter = useMemo<AnalyticsFilter>(
    () => ({ ...dimensions, startTime, endTime }),
    [dimensions, startTime, endTime]
  );

  const { data: metadata } = useAnalyticsMetadata();
  const { data: overview, isLoading: isOverviewLoading } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading } = useAnalyticsDailyStats(filter);
  const { data: channelStats, isLoading: isChannelLoading } = useAnalyticsDimensionStats(filter, 'channel');
  const { data: modelStats, isLoading: isModelLoading } = useAnalyticsDimensionStats(filter, 'model');
  const { data: apiKeyStats, isLoading: isApiKeyLoading } = useAnalyticsDimensionStats(filter, 'apiKey');
  // The user dimension is owner-only: gating the request rather than only the rendered
  // group keeps other users' per-user metrics from reaching a non-owner's browser.
  const { data: userStats, isLoading: isUserLoading } = useAnalyticsDimensionStats(filter, 'user', isProjectOwner);

  const isLoading = isChannelLoading || isModelLoading || isApiKeyLoading || isUserLoading;

  const groups = useMemo(
    () =>
      [
        { key: 'channel', label: t('analytics.table.channel'), data: channelStats || [] },
        { key: 'model', label: t('analytics.table.model'), data: modelStats || [] },
        { key: 'apiKey', label: t('analytics.table.apiKey'), data: apiKeyStats || [] },
        { key: 'user', label: t('analytics.table.user'), data: userStats || [] },
      ].filter((group) => group.key !== 'user' || isProjectOwner),
    [t, isProjectOwner, channelStats, modelStats, apiKeyStats, userStats]
  );

  const handleMetricChange = useCallback((value: string) => {
    setMetric(value as DistributionMetric);
    try {
      localStorage.setItem(METRIC_STORAGE_KEY, value);
    } catch {
      // Persisting the choice is a convenience; a full or blocked store must not break the page.
    }
  }, []);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Main fixed className='px-0 py-0'>
        <div className='flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-8 pt-6 pb-8'>
          <AnalyticsFilterBar earliestDate={metadata?.earliestDate} />
          <OverviewCards overview={overview} isLoading={isOverviewLoading} />
          <UsageCompositionChart data={dailyStats || []} isLoading={isDailyLoading} />

          <div className='flex flex-wrap items-center justify-end gap-2'>
            <Tabs value={metric} onValueChange={handleMetricChange}>
              <TabsList className='h-8'>
                {METRICS.map((value) => (
                  <TabsTrigger key={value} value={value} className='text-xs'>
                    {t(`analytics.distribution.metric.${value}`)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </div>

          <DimensionDistribution metric={metric} groups={groups} isLoading={isLoading} currencyCode={currencyCode} />
          <DimensionShareStrip metric={metric} groups={groups} isLoading={isLoading} currencyCode={currencyCode} />
          <DimensionDetailTable
            channelStats={channelStats || []}
            modelStats={modelStats || []}
            apiKeyStats={apiKeyStats || []}
            userStats={userStats || []}
            isLoading={isLoading}
          />
        </div>
      </Main>
    </div>
  );
}
