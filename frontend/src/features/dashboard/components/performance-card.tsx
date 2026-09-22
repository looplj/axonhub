import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatNumber } from '@/utils/format-number';
import { useChannelPerformanceStats, useModelPerformanceStats } from '../data/dashboard';
import { PerformanceChart, type PerformanceDataPoint } from './performance-chart';
import { RangeBadge } from './range-badge';

type PerformanceDimension = 'model' | 'channel';

/** Performance over time: throughput or TTFT per model or channel. */
export function PerformanceCard() {
  const { t } = useTranslation();
  const [dimension, setDimension] = useState<PerformanceDimension>('model');

  const { data: modelStats, isLoading: isModelLoading, error: modelError } = useModelPerformanceStats();
  const { data: channelStats, isLoading: isChannelLoading, error: channelError } = useChannelPerformanceStats();

  const isChannel = dimension === 'channel';

  const data: PerformanceDataPoint[] | undefined = isChannel
    ? channelStats?.map((stat) => ({
        date: stat.date,
        id: stat.channelId,
        name: stat.channelName,
        throughput: stat.throughput,
        ttftMs: stat.ttftMs,
        requestCount: stat.requestCount,
      }))
    : modelStats?.map((stat) => ({
        date: stat.date,
        id: stat.modelId,
        throughput: stat.throughput,
        ttftMs: stat.ttftMs,
        requestCount: stat.requestCount,
      }));

  const description = t('dashboard.charts.performanceDescription', {
    count: formatNumber((data ?? []).reduce((sum, item) => sum + item.requestCount, 0)),
  });

  return (
    <Card className='hover-card'>
      <CardHeader>
        <div className='space-y-1'>
          <CardTitle>{isChannel ? t('dashboard.charts.channelPerformance') : t('dashboard.charts.modelPerformance')}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
        <CardAction className='flex items-center gap-2'>
          <RangeBadge label={t('dashboard.charts.performanceRange')} />
          <Tabs value={dimension} onValueChange={(value) => setDimension(value as PerformanceDimension)}>
            <TabsList className='h-8'>
              <TabsTrigger value='model' className='text-xs'>
                {t('dashboard.stats.model')}
              </TabsTrigger>
              <TabsTrigger value='channel' className='text-xs'>
                {t('dashboard.stats.channel')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </CardAction>
      </CardHeader>
      <CardContent>
        <PerformanceChart
          data={data}
          isLoading={isChannel ? isChannelLoading : isModelLoading}
          error={isChannel ? channelError : modelError}
          emptyMessage={isChannel ? t('dashboard.charts.noChannelData') : t('dashboard.charts.noModelData')}
          errorMessage={isChannel ? t('dashboard.charts.errorLoadingChannelData') : t('dashboard.charts.errorLoadingModelData')}
          gradientPrefix={isChannel ? 'channel' : 'model'}
        />
      </CardContent>
    </Card>
  );
}
