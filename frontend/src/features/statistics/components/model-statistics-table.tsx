import { useTranslation } from 'react-i18next';
import { useModelStatistics } from '../data/statistics';
import type { StatisticsTimeWindow } from './time-filter';

interface ModelStatisticsTableProps {
  channelId?: string;
  timeWindow: StatisticsTimeWindow;
}

export function ModelStatisticsTable({ channelId, timeWindow }: ModelStatisticsTableProps) {
  const { t } = useTranslation();
  const { data: models, isLoading } = useModelStatistics(channelId, timeWindow);

  const formatNumber = (value: number | null | undefined): string => {
    if (value === null || value === undefined) return 'N/A';
    return value.toLocaleString();
  };

  const formatCurrency = (value: number | null | undefined): string => {
    if (value === null || value === undefined) return 'N/A';
    return value.toLocaleString('en-US', { style: 'currency', currency: 'USD' });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8 text-muted-foreground">
        {t('statistics.loading')}
      </div>
    );
  }

  if (!models || models.length === 0) {
    return (
      <div className="rounded-md border">
        <table className="w-full">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="p-2 text-left">{t('statistics.model')}</th>
              <th className="p-2 text-left">{t('statistics.channelName')}</th>
              <th className="p-2 text-right">{t('statistics.requests')}</th>
              <th className="p-2 text-right">{t('statistics.tokensIn')}</th>
              <th className="p-2 text-right">{t('statistics.tokensOut')}</th>
              <th className="p-2 text-right">{t('statistics.cached')}</th>
              <th className="p-2 text-right">{t('statistics.avgTtft')}</th>
              <th className="p-2 text-right">{t('statistics.avgTps')}</th>
              <th className="p-2 text-right">{t('statistics.cost')}</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td colSpan={9} className="p-4 text-center text-muted-foreground">
                {t('statistics.noData')}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    );
  }

  return (
    <div className="rounded-md border">
      <table className="w-full">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="p-2 text-left">{t('statistics.model')}</th>
            <th className="p-2 text-left">{t('statistics.channelName')}</th>
            <th className="p-2 text-right">{t('statistics.requests')}</th>
            <th className="p-2 text-right">{t('statistics.tokensIn')}</th>
            <th className="p-2 text-right">{t('statistics.tokensOut')}</th>
            <th className="p-2 text-right">{t('statistics.cached')}</th>
            <th className="p-2 text-right">{t('statistics.avgTtft')}</th>
            <th className="p-2 text-right">{t('statistics.avgTps')}</th>
            <th className="p-2 text-right">{t('statistics.cost')}</th>
          </tr>
        </thead>
        <tbody>
          {models.map((model) => (
            <tr key={model.modelId} className="border-b last:border-0 hover:bg-muted/30">
              <td className="p-2 font-medium">{model.modelId}</td>
              <td className="p-2">{model.channelName}</td>
              <td className="p-2 text-right">{model.requestCount.toLocaleString()}</td>
              <td className="p-2 text-right">{model.promptTokens.toLocaleString()}</td>
              <td className="p-2 text-right">{model.completionTokens.toLocaleString()}</td>
              <td className="p-2 text-right">{model.cachedTokens.toLocaleString()}</td>
              <td className="p-2 text-right">{formatNumber(model.avgTtftMs)}</td>
              <td className="p-2 text-right">{formatNumber(model.avgTps)}</td>
              <td className="p-2 text-right">{formatCurrency(model.totalCost)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
