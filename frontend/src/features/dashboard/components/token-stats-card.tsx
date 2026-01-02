import { useState } from 'react';
import { BarChart4 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useTokenStats } from '../data/dashboard';

type TimeRange = 'thisMonth' | 'thisWeek' | 'thisDay';

export function TokenStatsCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading, error } = useTokenStats();
  const [timeRange, setTimeRange] = useState<TimeRange>('thisMonth');

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <div className='space-y-2'>
            <Skeleton className='h-8 w-[80px]' />
            <Skeleton className='mt-1 h-4 w-[140px]' />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <div className='flex items-center gap-2'>
            <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
              <BarChart4 className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium'>{t('dashboard.cards.tokenStats')}</CardTitle>
          </div>
          <div className='flex items-center gap-1'>
            {/* <span className='text-xs text-muted-foreground'>{t('dashboard.stats.this')}</span> */}
            <span className='bg-primary/10 text-primary dark:bg-primary/20 rounded-md px-2 py-1 text-xs'>{t('dashboard.stats.month')}</span>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const getTokens = (range: TimeRange) => {
    if (range === 'thisDay') {
      return {
        input: stats?.totalInputTokensToday || 0,
        output: stats?.totalOutputTokensToday || 0,
        cached: stats?.totalCachedTokensToday || 0,
      };
    }
    if (range === 'thisMonth') {
      return {
        input: stats?.totalInputTokensThisMonth || 0,
        output: stats?.totalOutputTokensThisMonth || 0,
        cached: stats?.totalCachedTokensThisMonth || 0,
      };
    }
    return {
      input: stats?.totalInputTokensThisWeek || 0,
      output: stats?.totalOutputTokensThisWeek || 0,
      cached: stats?.totalCachedTokensThisWeek || 0,
    };
  };

  const tokens = getTokens(timeRange);

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
            <BarChart4 className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.tokenStats')}</CardTitle>
        </div>
        <div className='flex items-center gap-1'>
          {/* <span className='text-xs text-muted-foreground'>{t('dashboard.stats.this')}</span> */}
          <Tabs value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
            <TabsList className='h-6 p-0.5'>
              <TabsTrigger value='thisMonth' className='h-5 px-2 text-[10px]'>
                {t('dashboard.stats.month')}
              </TabsTrigger>
              <TabsTrigger value='thisWeek' className='h-5 px-2 text-[10px]'>
                {t('dashboard.stats.week')}
              </TabsTrigger>
              <TabsTrigger value='thisDay' className='h-5 px-2 text-[10px]'>
                {t('dashboard.stats.day')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </CardHeader>
      <CardContent>
        <div className='flex items-end justify-between'>
          <div className='text-center'>
            <div className='text-muted-foreground mb-1 text-xs'>{t('dashboard.stats.input')}</div>
            <div className='font-mono text-lg font-bold'>{formatNumber(tokens.input)}</div>
          </div>
          <div className='bg-border h-8 w-px'></div>
          <div className='text-center'>
            <div className='text-muted-foreground mb-1 text-xs'>{t('dashboard.stats.output')}</div>
            <div className='font-mono text-lg font-bold'>{formatNumber(tokens.output)}</div>
          </div>
          <div className='bg-border h-8 w-px'></div>
          <div className='text-center'>
            <div className='text-muted-foreground mb-1 text-xs'>{t('dashboard.stats.cached')}</div>
            <div className='text-muted-foreground font-mono text-lg font-bold'>{formatNumber(tokens.cached)}</div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
