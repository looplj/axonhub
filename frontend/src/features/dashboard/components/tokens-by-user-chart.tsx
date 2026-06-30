import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { Loader2 } from 'lucide-react';
import { formatNumber } from '@/utils/format-number';
import { Skeleton } from '@/components/ui/skeleton';
import { useGeneralSettings } from '../../system/data/system';
import { useUsageStatsByUser } from '../data/dashboard';
import type { TimePeriod } from '@/components/time-period-selector';
import { ChartLegend } from './chart-legend';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', 'var(--chart-6)'];

interface TokensByUserChartProps {
  timePeriod: TimePeriod;
}

export function TokensByUserChart({ timePeriod }: TokensByUserChartProps) {
  const { t, i18n } = useTranslation();

  const { data, isLoading, isFetching, error } = useUsageStatsByUser(timePeriod);
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = (val: number) =>
    t('currencies.format', {
      val,
      currency: currencyCode,
      locale,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });

  const { chartData, totalTokens, totalCost } = useMemo(() => {
    if (!data) return { chartData: [], totalTokens: 0, totalCost: 0 };

    const sorted = [...data].sort((a, b) => b.totalTokens - a.totalTokens);
    const top10 = sorted.slice(0, 10);

    const totalTok = top10.reduce((sum, item) => sum + item.totalTokens, 0);
    const totalC = top10.reduce((sum, item) => sum + item.totalCost, 0);

    return { chartData: top10, totalTokens: totalTok, totalCost: totalC };
  }, [data]);

  if (isLoading) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <Skeleton className='h-[250px] w-full rounded-md' />
      </div>
    );
  }

  type Payload = {
    name?: string;
    value?: number;
    payload?: {
      userName: string;
      requestCount: number;
      totalCost: number;
      totalTokens: number;
    };
  };

  type CombinedTooltipProps = TooltipProps<number, string> & {
    payload?: Payload[];
  };

  const tooltipContent = (props: CombinedTooltipProps) => {
    if (!props.active || !props.payload?.length) return null;
    const d = props.payload[0].payload;
    if (!d) return null;

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium mb-1'>{d.userName}</div>
        <div className='space-y-1'>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.tokenCount')}:</span>
            <span className='font-medium'>{formatNumber(d.totalTokens)} ({totalTokens ? ((d.totalTokens / totalTokens) * 100).toFixed(0) : 0}%)</span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.requestCount')}:</span>
            <span className='font-medium'>{formatNumber(d.requestCount)}</span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.userCost')}:</span>
            <span className='font-medium'>{formatCurrency(d.totalCost)} ({totalCost ? ((d.totalCost / totalCost) * 100).toFixed(0) : 0}%)</span>
          </div>
        </div>
      </div>
    );
  };

  const legendItems = chartData.map((item, index) => ({
    name: item.userName,
    index: index + 1,
    color: COLORS[index % COLORS.length],
    primaryValue: formatNumber(item.totalTokens),
    secondaryValue: formatCurrency(item.totalCost),
  }));

  return (
    <div className='relative space-y-6'>
      {error ? (
        <div className='flex h-[300px] items-center justify-center'>
          <div className='text-sm text-red-500'>
            {t('dashboard.charts.errorLoadingChart')} {error.message}
          </div>
        </div>
      ) : chartData.length === 0 ? (
        <div className='flex h-[300px] items-center justify-center'>
          <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noUserData')}</div>
        </div>
      ) : (
        <>
          <ResponsiveContainer width='100%' height={320}>
            <BarChart data={chartData} barSize={32}>
              <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
              <XAxis dataKey='userName' hide />
              <YAxis
                yAxisId='left'
                tickLine={false}
                axisLine={false}
                width={60}
                tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
                tickFormatter={(value) => {
                  if (value === 0) return '0';
                  const mValue = value / 1_000_000;
                  const formatted = mValue.toFixed(3).replace(/\.0+$|(?<=\.\d*[1-9])0+$/g, '');
                  return `${formatted}M`;
                }}
              />
              <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
              <Bar yAxisId='left' dataKey='totalTokens' radius={[6, 6, 0, 0]} isAnimationActive={false}>
                {chartData.map((_, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
          <ChartLegend items={legendItems} />
        </>
      )}
      {isFetching && (
        <div className='absolute inset-0 flex items-center justify-center bg-background/50'>
          <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
        </div>
      )}
    </div>
  );
}
