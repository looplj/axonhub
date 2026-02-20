import { useTranslation } from 'react-i18next';
import { PerformanceChart, PerformanceDataPoint } from './performance-chart';
import { useModelPerformanceStats } from '../data/dashboard';

interface ModelPerformanceStatsProps {
  onTotalRequestsChange?: (total: number) => void;
}

interface ModelPerformanceStat {
  modelId: string;
  throughput: number | null;
  ttftMs: number | null;
  requestCount: number;
  date: string;
}

export function ModelPerformanceStats({ onTotalRequestsChange }: ModelPerformanceStatsProps) {
  const { t } = useTranslation();
  const { data: performanceStats, isLoading, error } = useModelPerformanceStats();

  const mappedData: PerformanceDataPoint[] | undefined = performanceStats?.map((stat: ModelPerformanceStat) => ({
    id: stat.modelId,
    name: stat.modelId,
    throughput: stat.throughput,
    ttftMs: stat.ttftMs,
    requestCount: stat.requestCount,
    date: stat.date,
  }));

  return (
    <PerformanceChart
      data={mappedData}
      isLoading={isLoading}
      error={error}
      onTotalRequestsChange={onTotalRequestsChange}
      emptyMessage={t('dashboard.charts.noModelData')}
      errorMessage={t('dashboard.charts.errorLoadingModelData')}
      idField="modelId"
    />
  );
}
