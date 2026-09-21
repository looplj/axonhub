import { useTranslation } from 'react-i18next';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Globe } from 'lucide-react';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { useAnalyticsMetadata, useAnalyticsOverview, useAnalyticsDailyStats, useAnalyticsDimensionStats } from './data/analytics';
import { AnalyticsFilterBar } from './components/analytics-filter-bar';
import { OverviewCards } from './components/overview-cards';
import { CombinedTrendChart } from './components/combined-trend-chart';
import { DimensionPieCharts } from './components/dimension-pie-charts';
import { DimensionDetailTable } from './components/dimension-detail-table';
import { useGeneralSettings } from '@/features/system/data/system';

/** Analytics page: overview cards, trend chart and per-dimension breakdowns, all
 * driven by the shared filter bar. */
export default function AnalyticsPage() {
  const { t } = useTranslation();
  const filter = useAnalyticsFilterStore((state) => state.filter);
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';

  const { data: metadata } = useAnalyticsMetadata();
  const { data: overview, isLoading: isOverviewLoading } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading } = useAnalyticsDailyStats(filter);
  const { data: channelStats, isLoading: isChannelLoading } = useAnalyticsDimensionStats(filter, 'channel');
  const { data: modelStats, isLoading: isModelLoading } = useAnalyticsDimensionStats(filter, 'model');
  const { data: apiKeyStats, isLoading: isApiKeyLoading } = useAnalyticsDimensionStats(filter, 'apiKey');
  const { data: userStats, isLoading: isUserLoading } = useAnalyticsDimensionStats(filter, 'user');

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('analytics.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('analytics.description')}</p>
          </div>
          <span className='text-muted-foreground flex items-center gap-1.5 text-xs'>
            <Globe className='h-3.5 w-3.5' />
            {t('dashboard.scope.system')}
          </span>
        </div>
      </Header>

      <Main fixed>
        <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-auto'>
          <AnalyticsFilterBar earliestDate={metadata?.earliestDate} />
          <OverviewCards overview={overview} isLoading={isOverviewLoading} />
          <CombinedTrendChart data={dailyStats || []} isLoading={isDailyLoading} currencyCode={currencyCode} />
          <DimensionPieCharts
            channelStats={channelStats || []}
            modelStats={modelStats || []}
            apiKeyStats={apiKeyStats || []}
            userStats={userStats || []}
            isLoading={isChannelLoading || isModelLoading || isApiKeyLoading || isUserLoading}
            currencyCode={currencyCode}
          />
          <DimensionDetailTable
            channelStats={channelStats || []}
            modelStats={modelStats || []}
            apiKeyStats={apiKeyStats || []}
            userStats={userStats || []}
            isLoading={isChannelLoading || isModelLoading || isApiKeyLoading || isUserLoading}
          />
        </div>
      </Main>
    </div>
  );
}
