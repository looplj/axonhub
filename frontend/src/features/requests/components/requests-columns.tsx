'use client';

import { useCallback } from 'react';
import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { IconRoute, IconArrowsJoin2 } from '@tabler/icons-react';
import { zhCN, enUS } from 'date-fns/locale';
import { FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { formatDuration } from '@/utils/format-duration';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
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
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.id')} />,
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
        const originalModelId = request.modelID || t('requests.columns.unknown');

        // Check if there are any executions with different model IDs
        const executions = request.executions?.edges?.map((edge) => edge.node) || [];
        const executionModelIds = Array.from(new Set(executions.map((exe) => exe?.modelID || ''))).filter(
          (id) => id && id !== originalModelId
        );

        if (executionModelIds.length > 0) {
          return (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type='button'
                  className='flex w-fit cursor-help items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-800/50 dark:bg-amber-900/30 dark:text-amber-300 dark:hover:bg-amber-900/50'
                >
                  <span>{originalModelId}</span>
                  <IconRoute className='h-3.5 w-3.5 opacity-80' />
                </button>
              </TooltipTrigger>
              <TooltipContent side='right' className='border-amber-200 bg-white dark:bg-zinc-900'>
                <div className='flex items-center gap-2 p-2'>
                  <span className='text-muted-foreground text-xs whitespace-nowrap'>{t('requests.columns.executedModelId')}:</span>
                  <span className='rounded bg-amber-100 px-2 py-0.5 text-xs font-medium whitespace-nowrap text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'>
                    {executionModelIds[0]}
                  </span>
                </div>
              </TooltipContent>
            </Tooltip>
          );
        }

        return <div className='px-2 text-sm font-medium'>{originalModelId}</div>;
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
    {
      id: 'clientIP',
      accessorKey: 'clientIP',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.clientIP')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const clientIP = row.getValue('clientIP') as string;
        return <div className='font-mono text-xs'>{clientIP || '-'}</div>;
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
              const request = row.original;
              const channel = request.channel;

              if (!channel) {
                return <div className='text-muted-foreground font-mono text-xs'>-</div>;
              }

              // Check if there are any executions with different channels
              const executions = request.executions?.edges?.map((edge) => edge.node) || [];
              const executionChannels = executions
                .map((exe) => exe?.channel)
                .filter((c): c is { id: string; name: string } => !!c && c.id !== channel.id);

              const uniqueExecutionChannelNames = Array.from(new Set(executionChannels.map((c) => c.name)));

              if (uniqueExecutionChannelNames.length > 0) {
                return (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type='button'
                        className='flex w-fit cursor-help items-center gap-1.5 rounded-lg border border-rose-200 bg-rose-50 px-2 py-0.5 text-xs font-medium text-rose-700 transition-colors hover:bg-rose-100 dark:border-rose-800/50 dark:bg-rose-900/30 dark:text-rose-300 dark:hover:bg-rose-900/50'
                      >
                        <span>{channel.name}</span>
                        <IconArrowsJoin2 className='h-3.5 w-3.5 opacity-80' />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side='right' className='border-rose-200 bg-white dark:bg-zinc-900'>
                      <div className='flex flex-col gap-2 p-1'>
                        <div className='text-muted-foreground text-[10px] tracking-wider uppercase'>
                          {t('requests.columns.retryProcess')}
                        </div>
                        <div className='flex items-center gap-2 text-xs'>
                          <span className='text-muted-foreground bg-muted rounded px-1.5 py-0.5 whitespace-nowrap'>{channel.name}</span>
                          <IconRoute className='text-muted-foreground h-3 w-3 shrink-0' />
                          <span className='rounded bg-rose-100 px-1.5 py-0.5 font-medium whitespace-nowrap text-rose-800 dark:bg-rose-900/40 dark:text-rose-200'>
                            {uniqueExecutionChannelNames.join(', ')}
                          </span>
                        </div>
                      </div>
                    </TooltipContent>
                  </Tooltip>
                );
              }

              return <div className='px-2 font-mono text-xs'>{channel.name}</div>;
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
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.status')} />,
      cell: ({ row }) => {
        const status = row.getValue('status') as string;
        return <Badge className={getStatusColor(status)}>{t(`requests.status.${status}`)}</Badge>;
      },
      filterFn: (row, id, value) => {
        return value.includes(row.getValue(id));
      },
      enableSorting: false,
      enableHiding: false,
    },
    {
      id: 'tokens',
      accessorFn: (row) => {
        const usageLog = row.usageLogs?.edges?.[0]?.node;
        return (usageLog?.promptTokens || 0) + (usageLog?.completionTokens || 0);
      },
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
            <div className='text-sm font-medium'>
              {t('requests.columns.totalTokens')}
              {(totalTokens || 0).toLocaleString()}
            </div>
            <div className='text-muted-foreground'>
              {t('requests.columns.input')}: {promptTokens.toLocaleString()} | {t('requests.columns.output')}:{' '}
              {completionTokens.toLocaleString()}
            </div>
          </div>
        );
      },
      enableSorting: true,
      enableHiding: true,
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
      accessorFn: (row) => row.usageLogs?.edges?.[0]?.node?.promptCachedTokens || 0,
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
                rate: promptTokens > 0 ? ((cachedTokens / (promptTokens + cachedTokens)) * 100).toFixed(1) : '0.0',
              })}
            </div>
          </div>
        );
      },
      enableSorting: true,
      enableHiding: true,
      sortingFn: (rowA, rowB) => {
        const a = rowA.original.usageLogs?.edges?.[0]?.node?.promptCachedTokens || 0;
        const b = rowB.original.usageLogs?.edges?.[0]?.node?.promptCachedTokens || 0;
        return a - b;
      },
    },
    {
      id: 'writeCache',
      accessorFn: (row) => row.usageLogs?.edges?.[0]?.node?.promptWriteCachedTokens || 0,
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
      enableHiding: true,
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
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} />,
      cell: ({ row }) => {
        const date = new Date(row.getValue('createdAt'));
        return <div className='text-xs'>{format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}</div>;
      },
      enableSorting: false,
      enableHiding: false,
    },
  ];
  return columns;
}
