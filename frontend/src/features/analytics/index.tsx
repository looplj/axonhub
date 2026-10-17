import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Header } from '@/components/layout/header';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { useGeneralSettings } from '@/features/system/data/system';
import { availableCurrencyCodes, localeForLanguage } from '@/utils/currency';
import { useAnalyticsMetadata, useAnalyticsOverview, useAnalyticsDailyStats, useAnalyticsDimensionStats } from './data/analytics';
import { AnalyticsFilterBar } from './components/analytics-filter-bar';
import { OverviewCards } from './components/overview-cards';
import { CombinedTrendChart } from './components/combined-trend-chart';
import { DimensionPieCharts } from './components/dimension-pie-charts';
import { DimensionDetailTable } from './components/dimension-detail-table';

export default function AnalyticsPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const filter = useAnalyticsFilterStore((state) => state.filter);
  const { data: generalSettings } = useGeneralSettings();

  const { data: metadata } = useAnalyticsMetadata();
  const { data: overview, isLoading: isOverviewLoading } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading } = useAnalyticsDailyStats(filter);
  const { data: channelStats, isLoading: isChannelLoading } = useAnalyticsDimensionStats(filter, 'channel');
  const { data: modelStats, isLoading: isModelLoading } = useAnalyticsDimensionStats(filter, 'model');
  const { data: apiKeyStats, isLoading: isApiKeyLoading } = useAnalyticsDimensionStats(filter, 'apiKey');
  const { data: userStats, isLoading: isUserLoading } = useAnalyticsDimensionStats(filter, 'user');

  const currencyCodes = useMemo(
    () =>
      availableCurrencyCodes([
        overview?.totalCostByCurrency,
        ...(dailyStats ?? []).map((stat) => stat.costByCurrency),
        ...(channelStats ?? []).map((stat) => stat.costByCurrency),
        ...(modelStats ?? []).map((stat) => stat.costByCurrency),
        ...(apiKeyStats ?? []).map((stat) => stat.costByCurrency),
        ...(userStats ?? []).map((stat) => stat.costByCurrency),
      ]),
    [overview, dailyStats, channelStats, modelStats, apiKeyStats, userStats]
  );

  const [currencyCode, setCurrencyCode] = useState(generalSettings?.currencyCode || 'USD');

  useEffect(() => {
    if (currencyCodes.length === 0) return;
    if (!currencyCodes.includes(currencyCode)) {
      setCurrencyCode(currencyCodes[0]);
    }
  }, [currencyCodes, currencyCode]);

  return (
    <div className='flex-1 space-y-6 p-8 pt-6'>
      <Header />
      <Button onClick={() => navigate({ to: '/' })} variant='outline' className='self-start'>
        {t('dashboard.channelSuccessRates.backToDashboard')}
      </Button>
      <AnalyticsFilterBar earliestDate={metadata?.earliestDate} />
      {currencyCodes.length > 1 && (
        <div className='flex items-center gap-2'>
          <span className='text-muted-foreground text-sm'>{t('analytics.currency')}</span>
          <Select value={currencyCode} onValueChange={setCurrencyCode}>
            <SelectTrigger className='w-[220px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {currencyCodes.map((code) => (
                <SelectItem key={code} value={code}>
                  {t(`currencies.${code}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      <OverviewCards
        overview={overview}
        isLoading={isOverviewLoading}
        currencyCode={currencyCode}
        locale={localeForLanguage(i18n.language)}
      />
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
        currencyCode={currencyCode}
      />
    </div>
  );
}
