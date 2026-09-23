'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { Loader2 } from 'lucide-react';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatNumber } from '@/utils/format-number';
import { safeNumber, safeToFixed, sanitizeChartData, type ChartData } from '../utils/chart-helpers';
import { ChartLegend, type ChartLegendItem } from './chart-legend';
import { RangeBadge } from './range-badge';
import { useFastestChannels, useFastestModels } from '../data/fastest-performers';
import type { FastestChannel, FastestModel } from '../data/fastest-performers';
import { performanceWindowFromRange, type CoarseTimeWindow, type RelativeTimeWindow } from '../utils/time-window';

// 5 colors matches the slice limit in chartData processing (.slice(0, 5))
const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

interface HorizontalBarChartProps {
  data: ChartData[];
  total: number;
  height?: number;
  noDataLabel: string;
}

/** Horizontal throughput bars with a custom tooltip; labels are hidden because the
 * rows are rendered as a list beside the chart. */
function HorizontalBarChart({ data, total, height = 280, noDataLabel }: HorizontalBarChartProps) {
  const safeData = sanitizeChartData(data);
  const safeTotal = safeNumber(total);

  if (safeData.length === 0) {
    return (
      <div className='flex h-[250px] items-center justify-center text-muted-foreground text-sm'>
        {noDataLabel}
      </div>
    );
  }

  const tooltipContent = (props: TooltipProps<number, string>) => {
    const { active, payload } = props;
    if (!active || !payload?.length) return null;

    const item = payload[0].payload as ChartData;
    const safeThroughput = safeNumber(item.throughput);
    const percent = safeTotal > 0 ? (safeThroughput / safeTotal) * 100 : 0;

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium'>{item.name}</div>
        <div className='text-muted-foreground'>
          {safeToFixed(safeThroughput, 0)} tokens/s ({safeToFixed(percent, 0)}%)
        </div>
        <div className='text-muted-foreground text-xs'>
          {safeNumber(item.requestCount)} requests
        </div>
      </div>
    );
  };

  return (
    <ResponsiveContainer width='100%' height={height}>
      <BarChart data={safeData} layout='vertical' barSize={32} margin={{ left: 20, right: 20, top: 10, bottom: 10 }}>
        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' horizontal={false} />
        <XAxis type='number' hide />
        <YAxis type='category' dataKey='name' width={10} tick={false} tickLine={false} axisLine={false} />
        <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
        <Bar dataKey='throughput' radius={[0, 4, 4, 0]}>
          {safeData.map((_, index) => (
            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}

interface FastestPerformersCardProps {
  startTime: string | null;
  endTime: string | null;
  timeWindow?: RelativeTimeWindow;
}

type FastestDimension = 'channel' | 'model';

/** Throughput leaderboard with a model/channel switch; the time window is derived
 * from the shared time range filter. */
export function FastestPerformersCard({ startTime, endTime, timeWindow }: FastestPerformersCardProps) {
  const { t } = useTranslation();
  const [dimension, setDimension] = useState<FastestDimension>('channel');
  const coarseWindow: CoarseTimeWindow = performanceWindowFromRange(startTime, endTime, timeWindow);

  const channelsQuery = useFastestChannels(coarseWindow);
  const modelsQuery = useFastestModels(coarseWindow);

  const isModel = dimension === 'model';
  const { data, isLoading, isFetching, error } = isModel ? modelsQuery : channelsQuery;

  const title = isModel ? t('dashboard.cards.fastestPerformers.models') : t('dashboard.cards.fastestPerformers.channels');
  const description = t('dashboard.cards.fastestPerformers.description', {
    type: isModel ? t('dashboard.cards.fastestPerformers.modelType') : t('dashboard.cards.fastestPerformers.channelType'),
    count: formatNumber((data ?? []).reduce((sum, item) => sum + safeNumber(item.requestCount), 0)),
  });
  const noDataLabel = t('dashboard.cards.fastestPerformers.noData');

  const dimensionSwitch = (
    <Tabs value={dimension} onValueChange={(value) => setDimension(value as FastestDimension)}>
      <TabsList className='h-8'>
        <TabsTrigger value='channel' className='text-xs'>
          {t('dashboard.stats.channel')}
        </TabsTrigger>
        <TabsTrigger value='model' className='text-xs'>
          {t('dashboard.stats.model')}
        </TabsTrigger>
      </TabsList>
    </Tabs>
  );

  if (isLoading && !data) {
    return (
      <Card className='hover-card flex h-full flex-col'>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-5 w-[180px]' />
          {dimensionSwitch}
        </CardHeader>
        <CardContent className='flex-1'>
          <div className='flex h-[250px] items-center justify-center'>
            <Skeleton className='h-[200px] w-full' />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className='hover-card flex h-full flex-col'>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <CardTitle className='text-base font-medium'>{title}</CardTitle>
          {dimensionSwitch}
        </CardHeader>
        <CardContent className='flex-1'>
          <div className='text-sm text-red-500'>
            {t('common.loadError')}: {error.message}
          </div>
        </CardContent>
      </Card>
    );
  }

  const rows = (data ?? [])
    .filter((item) => item != null)
    .slice(0, 5)
    .map((item) => ({
      name: (isModel ? (item as FastestModel).modelName : (item as FastestChannel).channelName) || 'Unknown',
      throughput: safeNumber(item.throughput ?? 0),
      requestCount: safeNumber(item.requestCount ?? 0),
    }))
    .sort((a, b) => b.throughput - a.throughput);

  const total = rows.reduce((sum, row) => sum + row.throughput, 0);
  const totalRequests = rows.reduce((sum, row) => sum + row.requestCount, 0);

  const legendItems: ChartLegendItem[] = rows.map((row, index) => ({
    name: row.name,
    index: index + 1,
    color: COLORS[index % COLORS.length],
    primaryValue: `${safeToFixed(row.throughput, 0)} tok/s`,
    secondaryValue: `${formatNumber(row.requestCount)} req`,
  }));

  return (
    <Card className='hover-card flex h-full flex-col'>
      <CardHeader className='flex flex-row items-start justify-between space-y-0 pb-2'>
        <div className='space-y-1'>
          <CardTitle className='text-base font-medium'>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
        <CardAction className='flex items-center gap-2'>
          <RangeBadge timeWindow={coarseWindow} />
          {dimensionSwitch}
        </CardAction>
      </CardHeader>
      <CardContent className='relative flex-1'>
        <div className='space-y-4'>
          <HorizontalBarChart data={rows} total={total} noDataLabel={noDataLabel} />
          <ChartLegend items={legendItems} columns={1} />
        </div>
        {isFetching && (
          <div className='absolute inset-0 flex items-center justify-center bg-background/50'>
            <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
          </div>
        )}
      </CardContent>
    </Card>
  );
}
