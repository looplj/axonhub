import { useTranslation } from 'react-i18next';
import { useChannelStatistics } from '../data/statistics';
import type { StatisticsTimeWindow } from './time-filter';

interface ChannelStatisticsTableProps {
  timeWindow: StatisticsTimeWindow;
}

export function ChannelStatisticsTable({ timeWindow }: ChannelStatisticsTableProps) {
  const { t } = useTranslation();
  const { data: channels, isLoading } = useChannelStatistics(timeWindow);

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

  if (!channels || channels.length === 0) {
    return (
      <div className="rounded-md border">
        <table className="w-full">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="p-2 text-left">{t('statistics.channelName')}</th>
              <th className="p-2 text-left">{t('statistics.provider')}</th>
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
            <th className="p-2 text-left">{t('statistics.channelName')}</th>
            <th className="p-2 text-left">{t('statistics.provider')}</th>
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
          {channels.map((channel) => (
            <tr key={channel.channelId} className="border-b last:border-0 hover:bg-muted/30">
              <td className="p-2 font-medium">{channel.channelName}</td>
              <td className="p-2">
                <span className="inline-flex items-center rounded-full bg-muted px-2 py-1 text-xs font-medium">
                  {channel.channelType}
                </span>
              </td>
              <td className="p-2 text-right">{channel.requestCount.toLocaleString()}</td>
              <td className="p-2 text-right">{channel.promptTokens.toLocaleString()}</td>
              <td className="p-2 text-right">{channel.completionTokens.toLocaleString()}</td>
              <td className="p-2 text-right">{channel.cachedTokens.toLocaleString()}</td>
              <td className="p-2 text-right">{formatNumber(channel.avgTtftMs)}</td>
              <td className="p-2 text-right">{formatNumber(channel.avgTps)}</td>
              <td className="p-2 text-right">{formatCurrency(channel.totalCost)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
