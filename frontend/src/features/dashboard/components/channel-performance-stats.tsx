import { useTranslation } from 'react-i18next';
import { PerformanceChart } from './performance-chart';
import type { PerformanceDataPoint } from './performance-chart';
import { useChannelPerformanceStats } from '../data/dashboard';

interface ChannelPerformanceStatsProps {
  onTotalRequestsChange?: (total: number) => void;
}

export function ChannelPerformanceStats({ onTotalRequestsChange }: ChannelPerformanceStatsProps) {
  const { t } = useTranslation();
  const { data: performanceStats, isLoading, error } = useChannelPerformanceStats();

  const mappedData = performanceStats?.map((stat) => ({
    date: stat.date,
    id: stat.channelId,
    name: stat.channelName,
    throughput: stat.throughput,
    ttftMs: stat.ttftMs,
    requestCount: stat.requestCount,
  }));

  return (
    <PerformanceChart
      data={mappedData}
      isLoading={isLoading}
      error={error}
      onTotalRequestsChange={onTotalRequestsChange}
      emptyMessage={t('dashboard.charts.noChannelData')}
      errorMessage={t('dashboard.charts.errorLoadingChannelData')}
      idField="channelId"
      nameField="channelName"
    />
  );
}
