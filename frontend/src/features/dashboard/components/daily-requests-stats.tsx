'use client';

import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, Area, AreaChart } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Skeleton } from '@/components/ui/skeleton';
import { useDailyRequestStats } from '../data/dashboard';

export function DailyRequestStats() {
  const { t } = useTranslation();
  const { data: dailyStats, isLoading, error } = useDailyRequestStats();

  if (isLoading) {
    return (
      <div className='flex h-[350px] items-center justify-center'>
        <Skeleton className='h-full w-full' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex h-[350px] items-center justify-center text-red-500'>
        {t('dashboard.charts.errorLoadingChart')} {error.message}
      </div>
    );
  }

  // Transform data for the chart
  const chartData =
    dailyStats?.map((stat) => ({
      name: new Date(stat.date).toLocaleDateString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
      }),
      total: stat.count,
    })) || [];

  // Calculate max value for Y-axis domain
  const maxValue = Math.max(...chartData.map((d) => d.total), 0);
  const yAxisMax = Math.max(10, Math.ceil(maxValue * 1.1));

  return (
    <ResponsiveContainer width='100%' height={350}>
      <AreaChart data={chartData}>
        <defs>
          <linearGradient id='colorTotal' x1='0' y1='0' x2='0' y2='1'>
            <stop offset='5%' stopColor='var(--primary)' stopOpacity={0.2} />
            <stop offset='95%' stopColor='var(--primary)' stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
        <XAxis dataKey='name' stroke='var(--muted-foreground)' fontSize={12} tickLine={false} axisLine={false} />
        <YAxis
          stroke='var(--muted-foreground)'
          fontSize={12}
          tickLine={false}
          axisLine={false}
          domain={[0, yAxisMax]}
          tickFormatter={(value) => formatNumber(value)}
        />
        <Tooltip formatter={(value) => formatNumber(Number(value))} />
        <Area
          type='monotone'
          dataKey='total'
          stroke='var(--primary)'
          strokeWidth={2}
          fillOpacity={1}
          fill='url(#colorTotal)'
          dot={false}
          activeDot={{ r: 5 }}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
