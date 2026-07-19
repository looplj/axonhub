import { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Search, Loader2 } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Button } from '@/components/ui/button';
import { DateRangePicker, type DateTimeRangeValue } from '@/components/date-range-picker';
import { buildDateRangeWhereClause } from '@/utils/date-range';
import { formatNumber } from '@/utils/format-number';
import { useGeneralSettings } from '@/features/system/data/system';
import { useUsageStatsByUser } from './data/usage-stats';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

export default function UsageStatisticsPage() {
  const { t, i18n } = useTranslation();
  const [dateRange, setDateRange] = useState<DateTimeRangeValue | undefined>();
  const [searchTerm, setSearchTerm] = useState('');

  const timeWindowParam = useMemo(() => {
    if (!dateRange) return undefined;
    const where = buildDateRangeWhereClause(dateRange);
    if (!where.createdAtGTE && !where.createdAtLTE) return undefined;
    const fromStr = where.createdAtGTE || new Date(0).toISOString();
    const toStr = where.createdAtLTE || new Date().toISOString();
    return `custom:${fromStr},${toStr}`;
  }, [dateRange]);

  const { data, isLoading, isFetching, error } = useUsageStatsByUser(timeWindowParam);
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();

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
    return allData.filter((item) =>
      item.userName.toLowerCase().includes(searchTerm.toLowerCase())
    );
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
        <div className='flex items-center justify-between gap-4 mb-4 flex-shrink-0'>
          <div className='flex items-center gap-2'>
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
            <DateRangePicker value={dateRange} onChange={setDateRange} />
            {dateRange && (dateRange.from || dateRange.to) && (
              <Button
                variant='ghost'
                onClick={() => setDateRange(undefined)}
                className='h-8 px-2'
                size='sm'
              >
                {t('common.filters.reset')}
              </Button>
            )}
          </div>
        </div>

        <div className='shadow-soft relative flex-1 overflow-auto overflow-x-hidden rounded-2xl border border-[var(--table-border)]'>
          {filteredData.length === 0 ? (
            <div className='flex h-[200px] items-center justify-center bg-[var(--table-background)] rounded-2xl'>
              <div className='text-muted-foreground text-sm'>
                {searchTerm ? t('common.noResults') : t('dashboard.charts.noUserData')}
              </div>
            </div>
          ) : (
            <Table className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
              <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
                <TableRow className='group/row border-0'>
                  <TableHead className='w-12 text-center text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>#</TableHead>
                  <TableHead className='text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.user')}</TableHead>
                  <TableHead className='text-right text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.requestCount')}</TableHead>
                  <TableHead className='text-right text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.tokenCount')}</TableHead>
                  <TableHead className='text-right text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase'>{t('dashboard.stats.userCost')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className='space-y-1 !bg-[var(--table-background)] p-2'>
                {filteredData.map((item, index) => (
                  <TableRow
                    key={item.userId}
                    className='group/row table-row-hover rounded-xl border-0 !bg-[var(--table-background)] transition-all duration-200 ease-in-out'
                  >
                    <TableCell className='text-muted-foreground text-center text-xs border-0 bg-inherit px-4 py-3'>{index + 1}</TableCell>
                    <TableCell className='font-medium border-0 bg-inherit px-4 py-3'>{item.userName}</TableCell>
                    <TableCell className='text-right font-mono text-sm border-0 bg-inherit px-4 py-3'>{formatNumber(item.requestCount)}</TableCell>
                    <TableCell className='text-right font-mono text-sm border-0 bg-inherit px-4 py-3'>{formatNumber(item.totalTokens)}</TableCell>
                    <TableCell className='text-right font-mono text-sm border-0 bg-inherit px-4 py-3'>{formatCurrency(item.totalCost)}</TableCell>
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

