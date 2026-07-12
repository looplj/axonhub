import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { ChevronDown, BarChart4, Activity, DollarSign } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatCurrencySimple } from '../utils/format-currency';

function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}
import type { AnalyticsDimensionStat } from '../data/analytics';

interface CollapsibleSectionProps {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
  storageKey: string;
  defaultOpen?: boolean;
}

function CollapsibleSection({ title, icon, children, storageKey, defaultOpen = false }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(() => {
    try {
      const stored = localStorage.getItem(`analytics-section-${storageKey}`);
      return stored !== null ? stored === 'true' : defaultOpen;
    } catch {
      return defaultOpen;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(`analytics-section-${storageKey}`, isOpen.toString());
    } catch {
      // Silently fail
    }
  }, [isOpen, storageKey]);

  return (
    <div className='space-y-4'>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className='flex w-full items-center justify-between rounded-lg border bg-card p-4 text-left transition-colors hover:bg-accent/50'
      >
        <div className='flex items-center gap-3'>
          <div className='flex h-8 w-8 items-center justify-center rounded-md bg-primary/10'>
            {icon}
          </div>
          <span className='text-lg font-semibold'>{title}</span>
        </div>
        <motion.div
          animate={{ rotate: isOpen ? 180 : 0 }}
          transition={{ duration: 0.2, ease: 'easeInOut' }}
        >
          <ChevronDown className='h-5 w-5 text-muted-foreground' />
        </motion.div>
      </button>
      <AnimatePresence initial={false}>
        {isOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15, ease: 'easeInOut' }}
          >
            <div className='space-y-4'>{children}</div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

const COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-6)',
];

const MAX_ITEMS = 9;

interface PieChartCardProps {
  title: string;
  data: AnalyticsDimensionStat[];
  valueKey: 'totalTokens' | 'requestCount' | 'cost';
  valueFormatter: (val: number) => string;
  otherLabel: string;
}

function PieChartCard({ title, data, valueKey, valueFormatter, otherLabel }: PieChartCardProps) {
  const { t } = useTranslation();
  const legendRef = useRef<HTMLDivElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const [legendPortal, setLegendPortal] = useState<HTMLElement | null>(null);
  const [isCompact, setIsCompact] = useState(false);
  const total = data.reduce((sum, item) => sum + (item[valueKey] as number), 0);

  useEffect(() => {
    if (legendRef.current && legendRef.current !== legendPortal) {
      setLegendPortal(legendRef.current);
    }
  });

  useEffect(() => {
    const card = cardRef.current;
    if (!card) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const width = entry.contentRect.width;
        // 饼图200px + 图例约150px + 间距 = 400px
        setIsCompact(width < 450);
      }
    });

    observer.observe(card);
    return () => observer.disconnect();
  }, []);

  if (total === 0) {
    return (
      <Card className='hover-card'>
        <CardHeader className='pb-2'>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardContent className='flex h-[250px] items-center justify-center'>
          <p className='text-xs text-muted-foreground'>{t('analytics.table.noData')}</p>
        </CardContent>
      </Card>
    );
  }

  // Sort by value descending and take top N, but exclude items with < 2% ratio
  const sorted = [...data].sort((a, b) => (b[valueKey] as number) - (a[valueKey] as number));
  const chartData: Array<{ name: string; value: number; percentage: number }> = [];
  let otherValue = 0;

  for (const item of sorted) {
    const value = item[valueKey] as number;
    const percentage = total > 0 ? (value / total) * 100 : 0;
    if (chartData.length < MAX_ITEMS && percentage >= 2) {
      chartData.push({ name: item.name, value, percentage });
    } else {
      otherValue += value;
    }
  }

  const otherPercentage = total > 0 ? (otherValue / total) * 100 : 0;
  if (otherValue > 0) {
    chartData.push({ name: otherLabel, value: otherValue, percentage: otherPercentage });
  }

  // 按百分比降序排序图例
  chartData.sort((a, b) => b.percentage - a.percentage);

  const CustomTooltip = ({ active, payload }: { active?: boolean; payload?: Array<{ name: string; value: number }> }) => {
    if (!active || !payload || payload.length === 0) return null;
    const item = payload[0];
    const percentage = total > 0 ? ((item.value / total) * 100).toFixed(1) : '0';
    return (
      <div className='rounded-md border bg-background p-2 shadow-md text-xs'>
        <p className='font-medium'>{item.name}</p>
        <p className='text-muted-foreground'>
          {valueFormatter(item.value)} ({percentage}%)
        </p>
      </div>
    );
  };

  if (isCompact) {
    // 紧凑模式：饼图在上，图例在下
    return (
      <Card ref={cardRef} className='hover-card'>
        <CardHeader className='pb-2'>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='h-[200px]'>
            <ResponsiveContainer width='100%' height='100%'>
              <PieChart>
                <Pie
                  data={chartData}
                  cx='50%'
                  cy='50%'
                  innerRadius={40}
                  outerRadius={80}
                  paddingAngle={0}
                  dataKey='value'
                  nameKey='name'
                >
                  {chartData.map((_, index) => (
                    <Cell key={index} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip content={<CustomTooltip />} />
                <Legend
                  layout='vertical'
                  align='center'
                  verticalAlign='bottom'
                  iconSize={8}
                  itemSorter={null}
                  portal={legendPortal}
                  payload={chartData.map((item, index) => ({
                    value: item.name,
                    type: 'circle' as const,
                    color: COLORS[index % COLORS.length],
                  }))}
                  formatter={(value: string) => {
                    const item = chartData.find((d) => d.name === value);
                    const pct = item ? item.percentage.toFixed(1) : '0';
                    return <span className='text-xs'>{value} ({pct}%)</span>;
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div ref={legendRef} className='mt-2 flex flex-col items-center gap-1' />
        </CardContent>
      </Card>
    );
  }

  // 宽松模式：饼图在左，图例在右
  return (
    <Card ref={cardRef} className='hover-card'>
      <CardHeader className='pb-2'>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className='flex justify-center'>
          <div className='h-[200px] w-[200px]'>
            <ResponsiveContainer width='100%' height='100%'>
              <PieChart>
                <Pie
                  data={chartData}
                  cx='50%'
                  cy='50%'
                  innerRadius={40}
                  outerRadius={80}
                  paddingAngle={0}
                  dataKey='value'
                  nameKey='name'
                >
                  {chartData.map((_, index) => (
                    <Cell key={index} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip content={<CustomTooltip />} />
                <Legend
                  layout='vertical'
                  align='center'
                  verticalAlign='bottom'
                  iconSize={8}
                  itemSorter={null}
                  portal={legendPortal}
                  payload={chartData.map((item, index) => ({
                    value: item.name,
                    type: 'circle' as const,
                    color: COLORS[index % COLORS.length],
                  }))}
                  formatter={(value: string) => {
                    const item = chartData.find((d) => d.name === value);
                    const pct = item ? item.percentage.toFixed(1) : '0';
                    return <span className='text-xs'>{value} ({pct}%)</span>;
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div ref={legendRef} className='flex flex-col justify-center gap-1 pl-4' />
        </div>
      </CardContent>
    </Card>
  );
}

interface DimensionPieChartsProps {
  channelStats: AnalyticsDimensionStat[];
  modelStats: AnalyticsDimensionStat[];
  apiKeyStats: AnalyticsDimensionStat[];
  userStats: AnalyticsDimensionStat[];
  isLoading: boolean;
  currencyCode: string;
}

export function DimensionPieCharts({
  channelStats,
  modelStats,
  apiKeyStats,
  userStats,
  isLoading,
  currencyCode,
}: DimensionPieChartsProps) {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton key={i} className='h-[300px]' />
        ))}
      </div>
    );
  }

  const dimensions = [
    { title: t('analytics.table.channel'), data: channelStats },
    { title: t('analytics.table.model'), data: modelStats },
    { title: t('analytics.table.apiKey'), data: apiKeyStats },
    { title: t('analytics.table.user'), data: userStats },
  ];

  const metrics = [
    {
      key: 'totalTokens' as const,
      title: t('analytics.pie.tokenDistribution'),
      formatter: formatExactNumber,
      icon: <BarChart4 className='h-4 w-4 text-primary' />,
      storageKey: 'tokens',
    },
    {
      key: 'requestCount' as const,
      title: t('analytics.pie.requestDistribution'),
      formatter: formatExactNumber,
      icon: <Activity className='h-4 w-4 text-primary' />,
      storageKey: 'requests',
    },
    {
      key: 'cost' as const,
      title: t('analytics.pie.costDistribution'),
      formatter: (val: number) => formatCurrencySimple(val, currencyCode),
      icon: <DollarSign className='h-4 w-4 text-primary' />,
      storageKey: 'costs',
    },
  ];

  return (
    <div className='space-y-4'>
      {metrics.map((metric) => (
        <CollapsibleSection
          key={metric.key}
          title={metric.title}
          icon={metric.icon}
          storageKey={metric.storageKey}
          defaultOpen={true}
        >
          <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
            {dimensions.map((dim) => (
              <PieChartCard
                key={`${dim.title}-${metric.key}`}
                title={dim.title}
                data={dim.data}
                valueKey={metric.key}
                valueFormatter={metric.formatter}
                otherLabel={t('analytics.pie.other')}
              />
            ))}
          </div>
        </CollapsibleSection>
      ))}
    </div>
  );
}
