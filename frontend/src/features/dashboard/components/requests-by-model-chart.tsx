'use client';

import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Skeleton } from '@/components/ui/skeleton';
import { useRequestsByModel } from '../data/dashboard';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', 'var(--chart-1)'];

export function RequestsByModelChart() {
  const { t } = useTranslation();
  const { data: modelData, isLoading, error } = useRequestsByModel();

  if (isLoading) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <Skeleton className='h-[250px] w-full rounded-md' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <div className='text-sm text-red-500'>
          {t('dashboard.charts.errorLoadingModelData')} {error.message}
        </div>
      </div>
    );
  }

  if (!modelData || modelData.length === 0) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noModelData')}</div>
      </div>
    );
  }

  const total = modelData.reduce((sum, item) => sum + item.count, 0);
  const chartData = modelData
    .map((item) => ({
      name: item.modelId,
      value: item.count,
    }))
    .sort((a, b) => b.value - a.value);

  const legendItems = chartData.map((item, index) => ({
    ...item,
    index: index + 1,
    color: COLORS[index % COLORS.length],
    percent: total ? (item.value / total) * 100 : 0,
  }));

  type ModelTooltipProps = TooltipProps<number, string> & {
    payload?: Array<{
      name?: string;
      value?: number;
      payload?: {
        name: string;
        value: number;
      };
    }>;
  };

  const tooltipContent = (props: ModelTooltipProps) => {
    const payload = props.payload;

    if (!props.active || !payload?.length) return null;

    const [{ value }] = payload;
    const name = payload[0].payload?.name;
    const percent = total ? ((value ?? 0) / total) * 100 : 0;

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium'>{name}</div>
        <div className='text-muted-foreground'>
          {value?.toLocaleString()} ({percent.toFixed(0)}%)
        </div>
      </div>
    );
  };

  return (
    <div className='space-y-6'>
      <ResponsiveContainer width='100%' height={320}>
        <BarChart data={chartData} barSize={32}>
          <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
          <XAxis dataKey='name' hide />
          <YAxis tickLine={false} axisLine={false} width={60} tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }} />
          <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
          <Bar dataKey='value' radius={[6, 6, 0, 0]}>
            {chartData.map((_, index) => (
              <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>

      <div className='grid gap-4 sm:grid-cols-2'>
        {legendItems.map((item) => (
          <div key={item.name} className='grid w-full grid-cols-[auto_auto_1fr_auto] items-start gap-3'>
            <span className='text-muted-foreground w-8 text-right text-sm font-semibold tabular-nums'>
              {item.index.toString().padStart(2, '0')}.
            </span>
            <span className='mt-1 h-2.5 w-2.5 rounded-full' style={{ backgroundColor: item.color }} />
            <span className='text-foreground min-w-0 text-sm font-medium break-words'>{item.name}</span>
            <div className='text-right leading-tight'>
              <div className='text-foreground text-sm font-medium tabular-nums'>{formatNumber(item.value)}</div>
              <div className='text-muted-foreground text-xs tabular-nums'>{item.percent.toFixed(0)}%</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
