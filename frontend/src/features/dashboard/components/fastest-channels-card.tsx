'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { Loader2 } from 'lucide-react';
import { formatNumber } from '@/utils/format-number';
import { useFastestChannels } from '../data/fastest-performers';
import { safeNumber, safeToFixed, sanitizeChartData, type ChartData } from '../utils/chart-helpers';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

type TimeWindow = 'day' | 'week' | 'month';

function HorizontalBarChart({ data, total, height = 280, noDataLabel }: { data: ChartData[]; total: number; height?: number; noDataLabel: string }) {
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
          {safeToFixed(safeThroughput)} tokens/s ({safeToFixed(percent, 0)}%)
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
        <YAxis
          type='category'
          dataKey='name'
          width={10}
          tick={false}
          tickLine={false}
          axisLine={false}
        />
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

function ChartLegend({ items }: { items: Array<{ name: string; throughput: number; requestCount: number; color: string; index: number }> }) {
  return (
    <div className='grid gap-3'>
      {items.map((item) => (
        <div key={item.name} className='grid w-full grid-cols-[auto_auto_1fr_auto] items-center gap-3'>
          <span className='text-muted-foreground w-8 text-right text-sm font-semibold tabular-nums'>
            {item.index.toString().padStart(2, '0')}.
          </span>
          <span className='h-2.5 w-2.5 rounded-full' style={{ backgroundColor: item.color }} />
          <span className='text-foreground min-w-0 text-sm font-medium break-words'>{item.name}</span>
          <div className='text-right leading-tight'>
            <div className='text-foreground text-sm font-medium tabular-nums'>{safeToFixed(item.throughput)} tok/s</div>
            <div className='text-muted-foreground text-xs tabular-nums'>{formatNumber(safeNumber(item.requestCount))} req</div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function FastestChannelsCard() {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<TimeWindow>('day');

  const { data: channels, isLoading, isFetching, error } = useFastestChannels(timeWindow);

  if (isLoading && !channels) {
    return (
      <Card className='hover-card'>
        <CardHeader>
          <Skeleton className='h-5 w-[180px]' />
          <Skeleton className='h-4 w-[120px]' />
        </CardHeader>
        <CardContent>
          <div className='flex h-[250px] items-center justify-center'>
            <Skeleton className='h-[200px] w-full' />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className='hover-card'>
        <CardHeader>
          <CardTitle>{t('dashboard.cards.fastestPerformers.channels')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const channelData: ChartData[] = sanitizeChartData(
    (channels || [])
      .slice(0, 5)
      .filter((c) => c != null)
      .map((c) => ({
        name: c.channelName ?? 'Unknown',
        throughput: safeNumber(c.throughput),
        requestCount: safeNumber(c.requestCount),
      }))
  ).sort((a, b) => b.throughput - a.throughput);

  const channelTotal = channelData.reduce((sum, item) => sum + safeNumber(item.throughput), 0);
  const totalRequests = channelData.reduce((sum, item) => sum + item.requestCount, 0);

  const channelLegendItems = channelData.map((item, index) => ({
    ...item,
    index: index + 1,
    color: COLORS[index % COLORS.length],
  }));

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div>
          <CardTitle className='text-base font-medium'>{t('dashboard.cards.fastestPerformers.channels')}</CardTitle>
          <CardDescription>Fastest channels by throughput · {formatNumber(totalRequests)} total requests</CardDescription>
        </div>
        <div className='flex items-center gap-2'>
          <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as TimeWindow)}>
            <TabsList className='h-7 p-0.5'>
              <TabsTrigger value='month' className='h-6 px-2 text-[10px]'>
                {t('dashboard.stats.month')}
              </TabsTrigger>
              <TabsTrigger value='week' className='h-6 px-2 text-[10px]'>
                {t('dashboard.stats.week')}
              </TabsTrigger>
              <TabsTrigger value='day' className='h-6 px-2 text-[10px]'>
                {t('dashboard.stats.day')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </CardHeader>
      <CardContent className='relative'>
        <div className='space-y-4'>
          <HorizontalBarChart data={channelData} total={channelTotal} noDataLabel={t('dashboard.cards.fastestPerformers.noData')} />
          <ChartLegend items={channelLegendItems} />
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
