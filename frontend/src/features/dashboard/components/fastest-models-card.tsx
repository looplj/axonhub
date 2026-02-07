'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { useFastestModels } from '../data/fastest-performers';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

type TimeWindow = '1h' | '24h' | '7d';

interface ChartData {
  name: string;
  throughput: number;
  requestCount: number;
}

function HorizontalBarChart({ data, total }: { data: ChartData[]; total: number }) {
  const tooltipContent = (props: TooltipProps<number, string>) => {
    const { active, payload } = props;
    if (!active || !payload?.length) return null;

    const item = payload[0].payload as ChartData;
    const percent = total ? (item.throughput / total) * 100 : 0;

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium'>{item.name}</div>
        <div className='text-muted-foreground'>
          {item.throughput.toFixed(1)} tokens/s ({percent.toFixed(0)}%)
        </div>
        <div className='text-muted-foreground text-xs'>
          {item.requestCount} requests
        </div>
      </div>
    );
  };

  return (
    <ResponsiveContainer width='100%' height={250}>
      <BarChart data={data} layout='vertical' barSize={28} margin={{ left: 100, right: 20, top: 10, bottom: 10 }}>
        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' horizontal={false} />
        <XAxis type='number' hide />
        <YAxis
          type='category'
          dataKey='name'
          width={90}
          tick={{ fontSize: 11, fill: 'var(--muted-foreground)' }}
          tickLine={false}
          axisLine={false}
        />
        <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
        <Bar dataKey='throughput' radius={[0, 4, 4, 0]}>
          {data.map((_, index) => (
            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}

function ChartLegend({ items, total }: { items: Array<{ name: string; throughput: number; requestCount: number; color: string; index: number }>; total: number }) {
  return (
    <div className='grid gap-2'>
      {items.map((item) => (
        <div key={item.name} className='grid w-full grid-cols-[auto_auto_1fr_auto] items-center gap-2'>
          <span className='text-muted-foreground w-6 text-right text-xs font-semibold tabular-nums'>
            {item.index.toString().padStart(2, '0')}.
          </span>
          <span className='h-2 w-2 rounded-full' style={{ backgroundColor: item.color }} />
          <span className='text-foreground min-w-0 text-xs font-medium break-words truncate'>{item.name}</span>
          <div className='text-right leading-tight'>
            <div className='text-foreground text-xs font-medium tabular-nums'>{item.throughput.toFixed(1)} tok/s</div>
            <div className='text-muted-foreground text-[10px] tabular-nums'>{formatNumber(item.requestCount)} req</div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function FastestModelsCard() {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<TimeWindow>('24h');

  const { data: models, isLoading, error } = useFastestModels(timeWindow);

  if (isLoading) {
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
          <CardTitle>{t('dashboard.cards.fastestPerformers.models')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const modelData: ChartData[] = (models || [])
    .slice(0, 5)
    .map((m) => ({ name: m.modelName, throughput: m.throughput, requestCount: m.requestCount }))
    .sort((a, b) => b.throughput - a.throughput);

  const modelTotal = modelData.reduce((sum, item) => sum + item.throughput, 0);

  const modelLegendItems = modelData.map((item, index) => ({
    ...item,
    index: index + 1,
    color: COLORS[index % COLORS.length],
  }));

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div>
          <CardTitle className='text-base font-medium'>{t('dashboard.cards.fastestPerformers.models')}</CardTitle>
          <CardDescription>Fastest models by throughput</CardDescription>
        </div>
        <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as TimeWindow)}>
          <TabsList className='h-7 p-0.5'>
            <TabsTrigger value='1h' className='h-6 px-2 text-[10px]'>
              1h
            </TabsTrigger>
            <TabsTrigger value='24h' className='h-6 px-2 text-[10px]'>
              24h
            </TabsTrigger>
            <TabsTrigger value='7d' className='h-6 px-2 text-[10px]'>
              7d
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </CardHeader>
      <CardContent>
        {modelData.length > 0 ? (
          <div className='space-y-4'>
            <HorizontalBarChart data={modelData} total={modelTotal} />
            <ChartLegend items={modelLegendItems} total={modelTotal} />
          </div>
        ) : (
          <div className='text-muted-foreground text-sm'>{t('dashboard.cards.fastestPerformers.noData')}</div>
        )}
      </CardContent>
    </Card>
  );
}
