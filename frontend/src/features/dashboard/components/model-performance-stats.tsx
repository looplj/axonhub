import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { PerformanceChart, PerformanceDataPoint } from './performance-chart';
import { useModelPerformanceStats, ModelPerformanceStat } from '../data/dashboard';

interface ModelPerformanceStatsProps {
  onTotalRequestsChange?: (total: number) => void;
}

export function ModelPerformanceStats({ onTotalRequestsChange }: ModelPerformanceStatsProps) {
  const { t } = useTranslation();
  const { data: performanceStats, isLoading, error } = useModelPerformanceStats();

  const mappedData: PerformanceDataPoint[] | undefined = useMemo(() =>
    performanceStats?.map((stat: ModelPerformanceStat) => ({
      id: stat.modelId,
      name: stat.modelId,
      throughput: stat.throughput,
      ttftMs: stat.ttftMs,
      requestCount: stat.requestCount,
      date: stat.date,
    })),
    [performanceStats]
  );

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
