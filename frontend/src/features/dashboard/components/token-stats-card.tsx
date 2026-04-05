import { useState } from 'react';
import { BarChart4 } from 'lucide-react';
import { IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useTokenStats } from '../data/dashboard';

type TimeRange = 'allTime' | 'thisMonth' | 'thisWeek' | 'thisDay';

function formatLastUpdated(timestamp: string | null, locale: string): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  return date.toLocaleString(locale, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  });
}

interface LastUpdatedInfoProps {
  lastUpdated: string | null;
  locale: string;
  t: (key: string, options?: Record<string, string>) => string;
}

function LastUpdatedInfo({ lastUpdated, locale, t }: LastUpdatedInfoProps) {
  if (!lastUpdated) return null;

  const formattedTime = formatLastUpdated(lastUpdated, locale);
  const label = t('dashboard.stats.updated', { time: formattedTime });

  return (
    <>
      <div className='hidden sm:block w-6 h-6'>
        <TooltipProvider delayDuration={0}>
          <Tooltip>
            <TooltipTrigger asChild>
              <button className='text-muted-foreground hover:text-foreground transition-colors w-6 h-6 flex items-center justify-center'>
                <IconInfoCircle className='h-4 w-4' />
              </button>
            </TooltipTrigger>
            <TooltipContent>
              <span>{label}</span>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <div className='sm:hidden w-11 h-11'>
        <Popover>
          <PopoverTrigger asChild>
            <button className='text-muted-foreground hover:text-foreground transition-colors w-11 h-11 flex items-center justify-center'>
              <IconInfoCircle className='h-5 w-5' />
            </button>
          </PopoverTrigger>
          <PopoverContent className='w-fit'>
            <span className='text-sm'>{label}</span>
          </PopoverContent>
        </Popover>
      </div>
    </>
  );
}

export function TokenStatsCard() {
  const { t, i18n } = useTranslation();
  const { data: stats, isLoading, error } = useTokenStats();
  const [timeRange, setTimeRange] = useState<TimeRange>('thisDay');

  if (isLoading) {
    return (
      <Card className='min-w-0'>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <div className='flex items-end justify-between gap-2 sm:flex-col sm:gap-2 xl:flex-row xl:items-end xl:justify-between'>
            <div className='text-center w-full sm:min-w-0 sm:flex sm:items-center sm:justify-between xl:block xl:flex-1 xl:text-center'>
              <Skeleton className='h-4 w-[40px] sm:mb-0 xl:mb-1' />
              <Skeleton className='h-6 w-[60px]' />
            </div>
            <div className='bg-border h-8 w-px shrink-0 sm:hidden xl:block'></div>
            <div className='bg-border h-px w-full shrink-0 hidden sm:block xl:hidden'></div>
            <div className='text-center w-full sm:min-w-0 sm:flex sm:items-center sm:justify-between xl:block xl:flex-1 xl:text-center'>
              <Skeleton className='h-4 w-[40px] sm:mb-0 xl:mb-1' />
              <Skeleton className='h-6 w-[60px]' />
            </div>
            <div className='bg-border h-8 w-px shrink-0 sm:hidden xl:block'></div>
            <div className='bg-border h-px w-full shrink-0 hidden sm:block xl:hidden'></div>
            <div className='text-center w-full sm:min-w-0 sm:flex sm:items-center sm:justify-between xl:block xl:flex-1 xl:text-center'>
              <Skeleton className='h-4 w-[40px] sm:mb-0 xl:mb-1' />
              <Skeleton className='h-6 w-[60px]' />
            </div>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className='hover-card min-w-0'>
        <CardHeader className='flex flex-col sm:flex-row sm:items-center sm:justify-between space-y-2 sm:space-y-0 pb-2'>
          <div className='flex items-center gap-2 min-w-0'>
            <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5 shrink-0'>
              <BarChart4 className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium truncate'>{t('dashboard.cards.tokenStats')}</CardTitle>
          </div>
          <div className='flex items-center gap-1 shrink-0'>
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
    if (range === 'allTime') {
      return {
        input: stats?.totalInputTokensAllTime || 0,
        output: stats?.totalOutputTokensAllTime || 0,
        cached: stats?.totalCachedTokensAllTime || 0,
      };
    }
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
    <Card className='hover-card min-w-0'>
      <CardHeader className='flex flex-wrap items-start sm:items-center justify-between gap-2 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5 shrink-0'>
            <BarChart4 className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium whitespace-normal leading-tight'>{t('dashboard.cards.tokenStats')}</CardTitle>
        </div>
        <div className='flex items-center gap-2 shrink-0'>
          {/* <span className='text-xs text-muted-foreground'>{t('dashboard.stats.this')}</span> */}
          <Tabs value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
            <TabsList className='h-6 p-0.5'>
              <TabsTrigger value='allTime' className='h-5 px-2 text-[10px]'>
                {t('dashboard.stats.all')}
              </TabsTrigger>
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
          {timeRange === 'allTime' && (
            <LastUpdatedInfo
              lastUpdated={stats?.lastUpdated ?? null}
              locale={i18n.language}
              t={t}
            />
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className='flex items-end justify-between gap-2 sm:flex-col sm:gap-2 xl:flex-row xl:items-end xl:justify-between'>
          <div className='text-center min-w-0 sm:flex sm:items-center sm:justify-between sm:w-full xl:block xl:text-center xl:flex-1'>
            <div className='text-muted-foreground text-xs sm:mb-0 xl:mb-1'>{t('dashboard.stats.input')}</div>
            <div className='font-mono text-lg font-bold'>{formatNumber(tokens.input)}</div>
          </div>
          <div className='bg-border h-8 w-px shrink-0 sm:hidden xl:block'></div>
          <div className='bg-border h-px w-full shrink-0 hidden sm:block xl:hidden'></div>
          <div className='text-center min-w-0 sm:flex sm:items-center sm:justify-between sm:w-full xl:block xl:text-center xl:flex-1'>
            <div className='text-muted-foreground text-xs sm:mb-0 xl:mb-1'>{t('dashboard.stats.output')}</div>
            <div className='font-mono text-lg font-bold'>{formatNumber(tokens.output)}</div>
          </div>
          <div className='bg-border h-8 w-px shrink-0 sm:hidden xl:block'></div>
          <div className='bg-border h-px w-full shrink-0 hidden sm:block xl:hidden'></div>
          <div className='text-center min-w-0 sm:flex sm:items-center sm:justify-between sm:w-full xl:block xl:text-center xl:flex-1'>
            <div className='text-muted-foreground text-xs sm:mb-0 xl:mb-1'>{t('dashboard.stats.cached')}</div>
            <div className='text-muted-foreground font-mono text-lg font-bold'>{formatNumber(tokens.cached)}</div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
