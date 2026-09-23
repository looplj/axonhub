'use client';

import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { type ColumnDef, type SortingState, flexRender, getCoreRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from '@tanstack/react-table';
import { AlertTriangle, CheckCircle2, XCircle } from 'lucide-react';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { DataTablePagination } from '@/components/data-table-pagination';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { formatNumber } from '@/utils/format-number';
import { useDashboardTimeStore } from '@/stores/dashboardStore';
import { useChannelSuccessRates, useTokensByChannel, type ChannelSuccessRate } from '../data/dashboard';
import { coarseWindowFromRange } from '../utils/time-window';
import { RangeBadge } from '../components/range-badge';

interface ChannelSuccessRateRow extends ChannelSuccessRate {
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
}

/** Channel success rate details: sortable columns, type and warning filters, paging. */
export default function DashboardChannelSuccessRates() {
  const { t } = useTranslation();
  const { startTime, endTime, timeWindow } = useDashboardTimeStore();
  const coarseWindow = coarseWindowFromRange(startTime, endTime, timeWindow);

  const [sorting, setSorting] = useState<SortingState>([{ id: 'successRate', desc: true }]);
  const [typeFilter, setTypeFilter] = useState('all');
  const [warningsOnly, setWarningsOnly] = useState(false);

  const { data: channels, isLoading, error } = useChannelSuccessRates(undefined, coarseWindow);
  const { data: tokenStats } = useTokensByChannel(coarseWindow);

  const rows = useMemo<ChannelSuccessRateRow[]>(() => {
    const tokensByChannel = new Map((tokenStats ?? []).map((item) => [item.channelId, item]));
    return (channels ?? []).map((channel) => {
      const tokens = tokensByChannel.get(channel.channelId);
      return {
        ...channel,
        inputTokens: tokens?.inputTokens ?? 0,
        outputTokens: tokens?.outputTokens ?? 0,
        totalTokens: tokens?.totalTokens ?? 0,
      };
    });
  }, [channels, tokenStats]);

  const channelTypes = useMemo(() => {
    const types = new Set<string>();
    for (const channel of channels ?? []) {
      if (channel.channelType) types.add(channel.channelType);
    }
    return [...types].sort();
  }, [channels]);

  const filteredRows = useMemo(
    () =>
      rows.filter((row) => {
        if (typeFilter !== 'all' && row.channelType !== typeFilter) return false;
        if (warningsOnly && !row.channelDisabled) return false;
        return true;
      }),
    [rows, typeFilter, warningsOnly]
  );

  const columns = useMemo<ColumnDef<ChannelSuccessRateRow>[]>(
    () => [
      {
        accessorKey: 'channelName',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('analytics.table.name')} />,
        cell: ({ row }) => (
          <div className='flex min-w-0 items-center gap-2'>
            <span className='truncate font-medium' title={row.original.channelName}>
              {row.original.channelName || '-'}
            </span>
            {row.original.channelDisabled && <AlertTriangle className='h-3.5 w-3.5 shrink-0 text-red-500' />}
          </div>
        ),
      },
      {
        accessorKey: 'channelType',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.type')} />,
        cell: ({ row }) => <Badge variant='outline'>{row.original.channelType || '-'}</Badge>,
      },
      {
        accessorKey: 'totalCount',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortByTotal')} />,
        cell: ({ row }) => <div className='text-right font-mono tabular-nums'>{formatNumber(row.original.totalCount)}</div>,
      },
      {
        accessorKey: 'successCount',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortBySuccess')} />,
        cell: ({ row }) => (
          <div className='flex items-center justify-end gap-1.5 text-right font-mono tabular-nums text-green-600 dark:text-green-500'>
            <CheckCircle2 className='h-3.5 w-3.5' />
            {formatNumber(row.original.successCount)}
          </div>
        ),
      },
      {
        accessorKey: 'failedCount',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortByFailed')} />,
        cell: ({ row }) => (
          <div className='flex items-center justify-end gap-1.5 text-right font-mono tabular-nums text-red-500'>
            <XCircle className='h-3.5 w-3.5' />
            {formatNumber(row.original.failedCount)}
          </div>
        ),
      },
      {
        accessorKey: 'successRate',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortByRate')} />,
        cell: ({ row }) => (
          <div className='text-right'>
            <div className={`font-mono font-medium tabular-nums ${row.original.successRate < 90 ? 'text-red-500' : ''}`}>
              {row.original.successRate.toFixed(1)}%
            </div>
            <div className='mt-1 h-1.5 overflow-hidden rounded-full bg-muted'>
              <div className='h-full rounded-full bg-green-500' style={{ width: `${row.original.successRate}%` }} />
            </div>
          </div>
        ),
      },
      {
        accessorKey: 'inputTokens',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortByInputTokens')} />,
        cell: ({ row }) => <div className='text-right font-mono tabular-nums'>{formatNumber(row.original.inputTokens)}</div>,
      },
      {
        accessorKey: 'outputTokens',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortByOutputTokens')} />,
        cell: ({ row }) => <div className='text-right font-mono tabular-nums'>{formatNumber(row.original.outputTokens)}</div>,
      },
      {
        accessorKey: 'totalTokens',
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('dashboard.channelSuccessRates.sortByTotalTokens')} />,
        cell: ({ row }) => <div className='text-right font-mono font-medium tabular-nums'>{formatNumber(row.original.totalTokens)}</div>,
      },
    ],
    [t]
  );

  const table = useReactTable({
    data: filteredRows,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize: 20, pageIndex: 0 } },
  });

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex items-center gap-2'>
          <h2 className='text-xl font-bold tracking-tight'>{t('dashboard.channelSuccessRates.pageTitle')}</h2>
          <RangeBadge timeWindow={coarseWindow} />
        </div>
      </Header>

      <Main fixed>
        <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-auto'>
          <div className='flex flex-wrap items-center gap-2'>
            <Select value={typeFilter} onValueChange={setTypeFilter}>
              <SelectTrigger className='w-[160px]'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>{t('dashboard.channelSuccessRates.allTypes')}</SelectItem>
                {channelTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <label className='flex items-center gap-2 whitespace-nowrap text-sm'>
              <Checkbox checked={warningsOnly} onCheckedChange={(checked) => setWarningsOnly(checked === true)} />
              {t('dashboard.channelSuccessRates.showWarnings')}
            </label>
          </div>

          {error ? (
            <div className='text-sm text-red-500'>
              {t('common.loadError')} {error.message}
            </div>
          ) : (
            <>
              <div className='relative flex-1 overflow-auto rounded-xl border'>
                <Table>
                  <TableHeader className='sticky top-0 z-10 bg-card'>
                    {table.getHeaderGroups().map((headerGroup) => (
                      <TableRow key={headerGroup.id}>
                        {headerGroup.headers.map((header) => (
                          <TableHead key={header.id} className='whitespace-nowrap'>
                            {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                          </TableHead>
                        ))}
                      </TableRow>
                    ))}
                  </TableHeader>
                  <TableBody>
                    {isLoading ? (
                      <TableSkeleton rows={20} columns={columns.length} />
                    ) : table.getRowModel().rows?.length ? (
                      table.getRowModel().rows.map((row) => (
                        <TableRow key={row.id}>
                          {row.getVisibleCells().map((cell) => (
                            <TableCell key={cell.id} className='whitespace-nowrap'>
                              {flexRender(cell.column.columnDef.cell, cell.getContext())}
                            </TableCell>
                          ))}
                        </TableRow>
                      ))
                    ) : (
                      <TableRow>
                        <TableCell colSpan={columns.length} className='h-24 text-center'>
                          {t('common.noData')}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </div>

              <DataTablePagination table={table} />
            </>
          )}
        </div>
      </Main>
    </div>
  );
}
