'use client';

import { useCallback } from 'react';
import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { zhCN, enUS } from 'date-fns/locale';
import { FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { formatDuration } from '@/utils/format-duration';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import { Request } from '../data/schema';
import { getStatusColor } from './help';

export function useRequestsColumns(): ColumnDef<Request>[] {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const permissions = useRequestPermissions();
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 });

  // Define all columns
  const columns: ColumnDef<Request>[] = [
    {
      accessorKey: 'id',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.id')} />,
      cell: ({ row }) => {
        const handleClick = useCallback(() => {
          navigateWithSearch({
            to: '/project/requests/$requestId',
            params: { requestId: row.original.id },
          });
        }, [row.original.id, navigateWithSearch]);

        return (
          <button onClick={handleClick} className='text-primary cursor-pointer font-mono text-xs hover:underline'>
            #{extractNumberID(row.getValue('id'))}
          </button>
        );
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      id: 'modelId',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.modelId')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const request = row.original;
        return <div className='text-sm font-medium'>{request.modelID || t('requests.columns.unknown')}</div>;
      },
    },

    {
      id: 'stream',
      accessorKey: 'stream',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.stream')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const isStream = row.original.stream;
        return (
          <Badge
            className={
              isStream
                ? 'border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300'
                : 'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
            }
          >
            {isStream ? t('requests.stream.streaming') : t('requests.stream.nonStreaming')}
          </Badge>
        );
      },
      filterFn: (row, _id, value) => {
        return value.includes(row.original.stream?.toString() || '-');
      },
      enableHiding: true,
    },
    {
      id: 'source',
      accessorKey: 'source',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.source')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const source = row.getValue('source') as string;
        const sourceColors: Record<string, string> = {
          api: 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800',
          playground: 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800',
          test: 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800',
        };
        return (
          <Badge
            className={
              sourceColors[source] ||
              'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
            }
          >
            {t(`requests.source.${source}`)}
          </Badge>
        );
      },
      filterFn: (row, id, value) => {
        return value.includes(row.getValue(id));
      },
    },
    // Channel column - only show if user has permission to view channels
    ...(permissions.canViewChannels
      ? ([
          {
            id: 'channel',
            accessorFn: (row) => row.channel?.id || '',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.channel')} />,
            enableSorting: false,
            cell: ({ row }) => {
              const channel = row.original.channel;

              if (!channel) {
                return <div className='text-muted-foreground font-mono text-xs'>-</div>;
              }

              return <div className='font-mono text-xs'>{channel.name}</div>;
            },
            filterFn: (row, _id, value) => {
              // For client-side filtering, check if any of the selected channels match
              if (value.length === 0) return true; // No filter applied

              const channel = row.original.channel;
              if (!channel) return false;

              return value.includes(channel.id);
            },
          },
        ] as ColumnDef<Request>[])
      : []),
    // API Key column - only show if user has permission to view API keys
    ...(permissions.canViewApiKeys
      ? ([
          {
            accessorKey: 'apiKey',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.apiKey')} />,
            enableSorting: false,
            cell: ({ row }) => {
              return <div className='font-mono text-xs'>{row.original.apiKey?.name || '-'}</div>;
            },
          },
        ] as ColumnDef<Request>[])
      : []),

    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.status')} />,
      cell: ({ row }) => {
        const status = row.getValue('status') as string;
        return <Badge className={getStatusColor(status)}>{t(`requests.status.${status}`)}</Badge>;
      },
      filterFn: (row, id, value) => {
        return value.includes(row.getValue(id));
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'tokens',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.tokens')} />,
      cell: ({ row }) => {
        const request = row.original;
        const usageLog = request.usageLogs?.edges?.[0]?.node;

        if (!usageLog) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const promptTokens = usageLog.promptTokens || 0;
        const completionTokens = usageLog.completionTokens || 0;
        const totalTokens = promptTokens + completionTokens;

        return (
          <div className='space-y-0.5 text-xs'>
            <div className='text-sm font-medium'>{totalTokens.toLocaleString()}</div>
            <div className='text-muted-foreground'>
              {t('requests.columns.input')}: {promptTokens.toLocaleString()} | {t('requests.columns.output')}:{' '}
              {completionTokens.toLocaleString()}
            </div>
          </div>
        );
      },
      enableSorting: true,
      sortingFn: (rowA, rowB) => {
        const a =
          (rowA.original.usageLogs?.edges?.[0]?.node?.promptTokens || 0) +
          (rowA.original.usageLogs?.edges?.[0]?.node?.completionTokens || 0);
        const b =
          (rowB.original.usageLogs?.edges?.[0]?.node?.promptTokens || 0) +
          (rowB.original.usageLogs?.edges?.[0]?.node?.completionTokens || 0);
        return a - b;
      },
    },
    {
      id: 'readCache',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.readCache')} />,
      cell: ({ row }) => {
        const request = row.original;
        const usageLog = request.usageLogs?.edges?.[0]?.node;

        if (!usageLog) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const cachedTokens = usageLog.promptCachedTokens || 0;
        const promptTokens = usageLog.promptTokens || 0;

        if (cachedTokens === 0) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        return (
          <div className='text-xs'>
            <div className='text-sm font-medium'>{cachedTokens.toLocaleString()}</div>
            <div className='text-muted-foreground'>
              {t('requests.columns.cacheHitRate', {
                rate: promptTokens > 0 ? ((cachedTokens / promptTokens) * 100).toFixed(1) : '0.0',
              })}
            </div>
          </div>
        );
      },
      enableSorting: true,
      sortingFn: (rowA, rowB) => {
        const a = rowA.original.usageLogs?.edges?.[0]?.node?.promptCachedTokens || 0;
        const b = rowB.original.usageLogs?.edges?.[0]?.node?.promptCachedTokens || 0;
        return a - b;
      },
    },
    {
      id: 'writeCache',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.writeCache')} />,
      cell: ({ row }) => {
        const request = row.original;
        const usageLog = request.usageLogs?.edges?.[0]?.node;

        if (!usageLog) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const writeCachedTokens = usageLog.promptWriteCachedTokens || 0;
        const promptTokens = usageLog.promptTokens || 0;

        if (writeCachedTokens === 0) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        return (
          <div className='text-xs'>
            <div className='text-sm font-medium'>{writeCachedTokens.toLocaleString()}</div>
            <div className='text-muted-foreground'>
              {t('requests.columns.writeCacheRate', {
                rate: promptTokens > 0 ? ((writeCachedTokens / promptTokens) * 100).toFixed(1) : '0.0',
              })}
            </div>
          </div>
        );
      },
      enableSorting: true,
      sortingFn: (rowA, rowB) => {
        const a = rowA.original.usageLogs?.edges?.[0]?.node?.promptWriteCachedTokens || 0;
        const b = rowB.original.usageLogs?.edges?.[0]?.node?.promptWriteCachedTokens || 0;
        return a - b;
      },
    },
    {
      id: 'latency',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.latency')} />,
      cell: ({ row }) => {
        const request = row.original;
        const latencyParts = [];

        if (request.status === 'completed') {
          if (request.metricsLatencyMs != null) {
            latencyParts.push(formatDuration(request.metricsLatencyMs));
          }
          if (request.stream && request.metricsFirstTokenLatencyMs != null) {
            latencyParts.push(`TTFT: ${formatDuration(request.metricsFirstTokenLatencyMs)}`);
          }
        }

        if (latencyParts.length === 0) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        return <div className='font-mono text-xs'>{latencyParts.join(' | ')}</div>;
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'details',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.details')} />,
      cell: ({ row }) => {
        const handleViewDetails = () => {
          navigateWithSearch({
            to: '/project/requests/$requestId',
            params: { requestId: row.original.id },
          });
        };

        return (
          <Button variant='outline' size='sm' onClick={handleViewDetails}>
            <FileText className='mr-2 h-4 w-4' />
            {t('requests.actions.viewDetails')}
          </Button>
        );
      },
      enableHiding: true,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.createdAt')} />,
      cell: ({ row }) => {
        const date = new Date(row.getValue('createdAt'));
        return <div className='text-xs'>{format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}</div>;
      },
    },
  ];
  return columns;
}
