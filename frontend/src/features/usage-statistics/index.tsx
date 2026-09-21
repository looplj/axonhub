import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Search, Loader2 } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { TimeRangeFilter } from '@/components/time-range-filter';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { formatNumber } from '@/utils/format-number';
import { useGeneralSettings } from '@/features/system/data/system';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useAnalyticsDimensionStats, useAnalyticsMetadata, type AnalyticsFilter } from '@/features/analytics/data/analytics';
import type { TimeRangeValue } from '@/stores/dashboardStore';

type Dimension = 'user' | 'apiKey' | 'model' | 'channel';

const DIMENSIONS: Dimension[] = ['user', 'apiKey', 'model', 'channel'];

/** Usage statistics for the selected project, broken down by one dimension at a time
 * and scoped by the shared time range filter. */
export default function UsageStatisticsPage() {
  const { t, i18n } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const [range, setRange] = useState<TimeRangeValue>({ startTime: null, endTime: null });
  const [dimension, setDimension] = useState<Dimension>('user');
  const [searchTerm, setSearchTerm] = useState('');

  const { data: metadata } = useAnalyticsMetadata();
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();

  const filter = useMemo<AnalyticsFilter | null>(() => {
    if (!selectedProjectId) return null;
    return {
      projectIDs: [selectedProjectId],
      startTime: range.startTime,
      endTime: range.endTime,
    };
  }, [selectedProjectId, range.startTime, range.endTime]);

  const { data, isLoading, isFetching, error } = useAnalyticsDimensionStats(filter, dimension, !!selectedProjectId);

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

  const allData = useMemo(() => {
    if (!data) return [];
    return [...data].sort((a, b) => b.requestCount - a.requestCount);
  }, [data]);

  const filteredData = useMemo(() => {
    if (!allData) return [];
    if (!searchTerm) return allData;
    return allData.filter((item) => item.name.toLowerCase().includes(searchTerm.toLowerCase()));
  }, [allData, searchTerm]);

  if (isLoading || isSettingsLoading) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <Skeleton className='h-8 w-[200px]' />
        <Skeleton className='h-[400px] w-full' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <div className='text-red-500'>{t('common.loadError')} {error.message}</div>
      </div>
    );
  }

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('sidebar.items.usageStats')}</h2>
            <p className='text-sm text-muted-foreground'>{t('usageStats.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed className='flex flex-col'>
        <div className='flex flex-shrink-0 flex-col gap-3'>
          <TimeRangeFilter
            value={range}
            onChange={setRange}
            earliestDate={metadata?.earliestDate}
          />

          <div className='flex items-center justify-between gap-4'>
            <Tabs value={dimension} onValueChange={(v) => setDimension(v as Dimension)}>
              <TabsList className='h-8'>
                {DIMENSIONS.map((value) => (
                  <TabsTrigger key={value} value={value} className='text-xs'>
                    {t(`analytics.table.${value}`)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            <div className='relative w-72'>
              <Search className='absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground' />
              <Input
                type='search'
                placeholder={t('search.placeholder')}
                className='h-8 pl-8'
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
            </div>
          </div>
        </div>

        <div className='shadow-soft relative mt-4 flex-1 overflow-auto overflow-x-hidden rounded-2xl border border-[var(--table-border)]'>
          {filteredData.length === 0 ? (
            <div className='flex h-[200px] items-center justify-center bg-[var(--table-background)] rounded-2xl'>
              <div className='text-muted-foreground text-sm'>
                {searchTerm ? t('common.noResults') : t('analytics.table.noData')}
              </div>
            </div>
          ) : (
            <Table className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
              <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
                <TableRow className='group/row border-0'>
                  <TableHead className='w-12 text-center text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>#</TableHead>
                  <TableHead className='text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('analytics.table.name')}</TableHead>
                  <TableHead className='text-right text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.requestCount')}</TableHead>
                  <TableHead className='text-right text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.tokenCount')}</TableHead>
                  <TableHead className='text-right text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.userCost')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className='space-y-1 !bg-[var(--table-background)] p-2'>
                {filteredData.map((item, index) => (
                  <TableRow
                    key={item.id}
                    className='group/row table-row-hover rounded-xl border-0 !bg-[var(--table-background)] transition-all duration-200 ease-in-out'
                  >
                    <TableCell className='text-muted-foreground text-center text-xs border-0 bg-inherit px-4 py-3'>{index + 1}</TableCell>
                    <TableCell className='font-medium border-0 bg-inherit px-4 py-3'>{item.name}</TableCell>
                    <TableCell className='text-right font-mono text-sm border-0 bg-inherit px-4 py-3'>{formatNumber(item.requestCount)}</TableCell>
                    <TableCell className='text-right font-mono text-sm border-0 bg-inherit px-4 py-3'>{formatNumber(item.totalTokens)}</TableCell>
                    <TableCell className='text-right font-mono text-sm border-0 bg-inherit px-4 py-3'>{formatCurrency(item.cost)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          {isFetching && (
            <div className='absolute inset-0 flex items-center justify-center bg-background/50 rounded-2xl z-30'>
              <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
            </div>
          )}
        </div>
      </Main>
    </div>
  );
}
