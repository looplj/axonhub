'use client';

import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { IconArrowsJoin2, IconRoute } from '@tabler/icons-react';
import { Ban, FileText } from 'lucide-react';
import { zhCN, enUS } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberID } from '@/lib/utils';
import { formatDuration } from '@/utils/format-duration';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { useGeneralSettings, useSecuritySettings, useUpdateSecuritySettings } from '@/features/system/data/system';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import { Request } from '../data/schema';
import { calculateTokensPerSecond, getTokensPerSecondValue } from '../utils/tokens-per-second';
import { getStatusColor } from './help';

interface UseRequestsColumnsOptions {
  onBodyClick?: (requestId: string, index: number) => void;
  onViewDetail?: (requestId: string) => void;
}

export const DEFAULT_HIDDEN_COLUMN_IDS = ['status', 'source', 'apiFormat', 'clientIP', 'tokensPerSecond', 'writeCache'];

export const DEFAULT_MOBILE_HIDDEN_COLUMN_IDS = [
  ...DEFAULT_HIDDEN_COLUMN_IDS,
  'channel',
  'tokens',
  'readCache',
  'writeCache',
  'cost',
  'duration',
  'caller',
];

export const MODEL_ID_COLUMN = 'modelID' as const;

export function useRequestsColumns(options?: UseRequestsColumnsOptions): ColumnDef<Request>[] {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const permissions = useRequestPermissions();
  const { hasSystemScope } = usePermissions();
  const { data: settings } = useGeneralSettings();
  const { data: securitySettings } = useSecuritySettings();
  const updateSecuritySettings = useUpdateSecuritySettings();
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 });
  const canManageSecuritySettings = hasSystemScope('write_settings');

  const blockedIPs = securitySettings?.blockedIPs ?? [];
  const showIPBanIcon = securitySettings?.showRequestLogIPBanIcon === true;

  const normalizeBlockedIPs = (ips: string[]) => Array.from(new Set(ips.map((ip) => ip.trim()).filter((ip) => ip.length > 0)));

  const handleBlockIP = async (clientIP: string) => {
    const normalizedIP = clientIP.trim();
    if (!normalizedIP) return;

    const nextBlockedIPs = normalizeBlockedIPs([...blockedIPs, normalizedIP]);
    if (nextBlockedIPs.length === blockedIPs.length && blockedIPs.includes(normalizedIP)) {
      toast.info(t('requests.actions.ipAlreadyBlocked'));
      return;
    }

    await updateSecuritySettings.mutateAsync({ blockedIPs: nextBlockedIPs });
  };

  const handleUnblockIP = async (clientIP: string) => {
    const normalizedIP = clientIP.trim();
    if (!normalizedIP) return;

    await updateSecuritySettings.mutateAsync({ blockedIPs: blockedIPs.filter((ip) => ip.trim() !== normalizedIP) });
  };

  const openDetail = (requestId: string) => {
    if (options?.onViewDetail) {
      options.onViewDetail(requestId);
      return;
    }

    navigateWithSearch({
      to: '/project/requests/$requestId',
      params: { requestId },
    });
  };

  const columns: ColumnDef<Request>[] = [
    {
      accessorKey: 'id',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.id')} />,
      enableSorting: true,
      enableHiding: false,
      cell: ({ row }) => {
        const request = row.original;
        const isStream = request.stream;

        return (
          <div className='flex min-w-[120px] flex-col gap-1.5'>
            <button
              type='button'
              onClick={() => options?.onBodyClick?.(request.id, row.index)}
              className='text-primary w-fit cursor-pointer font-mono text-xs hover:underline'
            >
              #{extractNumberID(request.id)}
            </button>
            <div className='flex flex-wrap items-center gap-1.5'>
              <Badge className={`${getStatusColor(request.status)} w-fit`}>{t(`requests.status.${request.status}`)}</Badge>
              <Badge
                className={
                  isStream
                    ? 'border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300'
                    : 'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
                }
              >
                {isStream ? t('requests.stream.streaming') : t('requests.stream.nonStreaming')}
              </Badge>
            </div>
          </div>
        );
      },
    },
    {
      id: 'status',
      accessorKey: 'status',
      enableHiding: false,
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
      cell: () => null,
    },
    {
      id: 'modelID',
      accessorFn: (row) => row.modelID,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.model')} />,
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const request = row.original;
        const originalModelId = request.modelID || t('requests.columns.unknown');
        const executions = request.executions?.edges?.flatMap((edge) => (edge.node ? [edge.node] : [])) ?? [];
        const executionModelIds = Array.from(new Set(executions.map((exe) => exe.modelID || ''))).filter(
          (id) => id && id !== originalModelId
        );
        const reasoningEffort = executions[0]?.reasoningEffort ?? request.reasoningEffort;
        const passThroughApplied = executions.some((execution) => execution.passThroughApplied);

        const modelLabel =
          executionModelIds.length > 0 ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type='button'
                  className='flex w-fit cursor-help items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-2 py-0.5 font-mono text-xs font-medium text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-800/50 dark:bg-amber-900/30 dark:text-amber-300 dark:hover:bg-amber-900/50'
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
          ) : (
            <span className='font-mono text-xs font-medium'>{originalModelId}</span>
          );

        return (
          <div className='flex min-w-[160px] flex-col gap-1'>
            {modelLabel}
            <div className='flex items-center gap-1.5'>
              {reasoningEffort && (
                <Badge className='border-sky-200 bg-sky-100 text-sky-800 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300'>
                  {reasoningEffort}
                </Badge>
              )}
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className={`inline-flex h-5 w-5 items-center justify-center ${
                      passThroughApplied ? 'text-amber-700 dark:text-amber-300' : 'text-muted-foreground/45'
                    }`}
                    tabIndex={0}
                    role='img'
                    aria-label={t(passThroughApplied ? 'requests.tooltips.passThroughApplied' : 'requests.tooltips.passThroughNotApplied')}
                  >
                    <IconRoute className='h-3.5 w-3.5' />
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t(passThroughApplied ? 'requests.tooltips.passThroughApplied' : 'requests.tooltips.passThroughNotApplied')}</TooltipContent>
              </Tooltip>
            </div>
          </div>
        );
      },
    },
    {
      id: 'apiFormat',
      accessorFn: (row) => row.format,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.apiFormat')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const format = row.original.format;
        return format ? (
          <span className='inline-flex items-center rounded-md border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-700 dark:border-zinc-700 dark:bg-zinc-800/50 dark:text-zinc-300'>
            {format}
          </span>
        ) : (
          <span className='text-muted-foreground text-xs'>-</span>
        );
      },
    },
    {
      id: 'source',
      accessorKey: 'source',
      enableHiding: false,
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
      cell: () => null,
    },
    {
      id: 'clientIP',
      accessorKey: 'clientIP',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.clientIP')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const normalizedIP = row.original.clientIP?.trim() ?? '';
        if (!normalizedIP) return <span className='text-muted-foreground text-xs'>-</span>;

        const isBlocked = blockedIPs.includes(normalizedIP);
        return (
          <div className='flex items-center gap-2'>
            <span className='font-mono text-xs'>{normalizedIP}</span>
            {canManageSecuritySettings &&
              showIPBanIcon &&
              (isBlocked ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      className='h-6 w-6 shrink-0 text-red-500/80 hover:bg-red-50 hover:text-red-600 dark:text-red-300/80 dark:hover:bg-red-950/30 dark:hover:text-red-300'
                      disabled={updateSecuritySettings.isPending}
                      onClick={() => void handleUnblockIP(normalizedIP)}
                      aria-label={t('requests.actions.unblockIP')}
                    >
                      <Ban className='h-3.5 w-3.5' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('requests.actions.unblockIP')}</TooltipContent>
                </Tooltip>
              ) : (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      className='text-muted-foreground h-6 w-6 shrink-0 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30 dark:hover:text-red-300'
                      disabled={updateSecuritySettings.isPending}
                      onClick={() => void handleBlockIP(normalizedIP)}
                      aria-label={t('requests.actions.blockIP')}
                    >
                      <Ban className='h-3.5 w-3.5' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('requests.actions.blockIP')}</TooltipContent>
                </Tooltip>
              ))}
          </div>
        );
      },
    },
    ...(permissions.canViewChannels
      ? ([
          {
            id: 'channel',
            accessorFn: (row) => row.executions?.edges?.[0]?.node?.channel?.id ?? row.channel?.id ?? '',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.channel')} />,
            enableSorting: false,
            enableHiding: true,
            cell: ({ row }) => {
              const request = row.original;
              const executions = request.executions?.edges?.flatMap((edge) => (edge.node ? [edge.node] : [])) ?? [];
              const finalExecution = executions[0];
              const channel = finalExecution?.channel ?? request.channel;

              if (!channel) return <span className='text-muted-foreground font-mono text-xs'>-</span>;

              const hasMultipleChannels = executions.some((exe) => exe.channel?.id && exe.channel.id !== channel.id);

              if (executions.length > 1 || hasMultipleChannels) {
                const sortedExecutions = [...executions].sort((a, b) => {
                  const dateA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
                  const dateB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
                  return dateB - dateA;
                });

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
                    <TooltipContent side='right' className='border-rose-200 bg-white p-0 dark:bg-zinc-900'>
                      <div className='flex min-w-[240px] flex-col'>
                        <div className='flex flex-col gap-1 border-b bg-rose-50/50 p-3 dark:bg-rose-900/10'>
                          <div className='flex items-center gap-2 text-xs font-bold tracking-wider text-rose-900 uppercase dark:text-rose-300'>
                            <IconArrowsJoin2 className='h-3.5 w-3.5' />
                            {t('requests.columns.retryProcess')}
                          </div>
                        </div>
                        <div className='flex flex-col gap-1 p-2'>
                          {sortedExecutions.map((exe, idx) => (
                            <div
                              key={exe.id || idx}
                              className='hover:bg-muted/50 flex items-center gap-2 rounded-md px-2 py-1.5 transition-colors'
                            >
                              <Badge className={`${getStatusColor(exe.status || '')} h-5 shrink-0 px-1.5 text-[10px] font-bold uppercase`}>
                                {exe.status ? t(`requests.status.${exe.status}`) : t('requests.columns.unknown')}
                              </Badge>
                              <div className='flex min-w-0 flex-col'>
                                <span className='text-foreground truncate text-xs font-semibold'>
                                  {exe.channel?.name || t('requests.columns.unknown')}
                                </span>
                                {exe.createdAt && (
                                  <span className='text-muted-foreground text-[10px]'>
                                    {format(new Date(exe.createdAt), 'HH:mm:ss', { locale })}
                                  </span>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    </TooltipContent>
                  </Tooltip>
                );
              }

              return <span className='font-mono text-xs'>{channel.name}</span>;
            },
            filterFn: (row, _id, value) => {
              if (value.length === 0) return true;
              const channel = row.original.executions?.edges?.[0]?.node?.channel ?? row.original.channel;
              return !!channel?.id && value.includes(channel.id);
            },
          },
        ] as ColumnDef<Request>[])
      : []),
    {
      id: 'tokens',
      accessorFn: (row) => {
        const usageLog = row.usageLogs?.edges?.[0]?.node;
        return (usageLog?.promptTokens || 0) + (usageLog?.completionTokens || 0);
      },
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.tokens')} />,
      cell: ({ row }) => {
        const usageLog = row.original.usageLogs?.edges?.[0]?.node;

        if (!usageLog) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const promptTokens = usageLog.promptTokens || 0;
        const completionTokens = usageLog.completionTokens || 0;
        const reasoningTokens = usageLog.completionReasoningTokens || 0;
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
            {reasoningTokens > 0 && (
              <div className='text-muted-foreground'>
                {t('requests.columns.reasoning')}: {reasoningTokens.toLocaleString()}
              </div>
            )}
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
        const usageLog = row.original.usageLogs?.edges?.[0]?.node;

        if (!usageLog) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const cachedTokens = usageLog.promptCachedTokens || 0;
        const promptTokens = usageLog.promptTokens || 0;

        if (cachedTokens === 0) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const hitRate = promptTokens > 0 ? (cachedTokens / promptTokens) * 100 : 0;
        const isLowHitRate = hitRate < 80 && promptTokens >= 40000;

        return (
          <div className='text-xs'>
            <div className='text-sm font-medium'>{cachedTokens.toLocaleString()}</div>
            <div className={isLowHitRate ? 'font-medium text-red-600 dark:text-red-400' : 'text-muted-foreground'}>
              {t('requests.columns.cacheHitRate', {
                rate: hitRate.toFixed(1),
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
        const usageLog = row.original.usageLogs?.edges?.[0]?.node;

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
      id: 'cost',
      accessorFn: (row) => row.usageLogs?.edges?.[0]?.node?.totalCost ?? null,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.cost')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const cost = row.original.usageLogs?.edges?.[0]?.node?.totalCost;
        if (cost == null) return <span className='font-mono text-xs'>-</span>;

        return (
          <span className='font-mono text-xs font-medium'>
            {t('currencies.format', {
              val: cost,
              currency: settings?.currencyCode ?? 'USD',
              locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
              minimumFractionDigits: 6,
            })}
          </span>
        );
      },
    },
    {
      id: 'duration',
      accessorFn: (row) => row.metricsLatencyMs ?? null,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.duration')} />,
      enableSorting: true,
      enableHiding: true,
      cell: ({ row }) => {
        const request = row.original;
        if (request.status !== 'completed' || request.metricsLatencyMs == null) {
          return <span className='text-muted-foreground text-xs'>-</span>;
        }

        if (!request.stream) {
          return <span className='font-mono text-xs'>{t('requests.duration.total', { duration: formatDuration(request.metricsLatencyMs) })}</span>;
        }

        return (
          <div className='min-w-[128px] font-mono text-xs'>
            {request.metricsFirstTokenLatencyMs != null && <div>{t('requests.duration.firstToken', { duration: formatDuration(request.metricsFirstTokenLatencyMs) })}</div>}
            <div className='text-muted-foreground'>{t('requests.duration.total', { duration: formatDuration(request.metricsLatencyMs) })}</div>
          </div>
        );
      },
      sortingFn: (rowA, rowB) => (rowA.original.metricsLatencyMs ?? 0) - (rowB.original.metricsLatencyMs ?? 0),
    },
    {
      id: 'tokensPerSecond',
      accessorFn: (row) => getTokensPerSecondValue(row) ?? 0,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.tokensPerSecond')} />,
      enableSorting: true,
      enableHiding: true,
      cell: ({ row }) => <span className='font-mono text-xs'>{calculateTokensPerSecond(row.original)}</span>,
      sortingFn: (rowA, rowB) => (getTokensPerSecondValue(rowA.original) ?? 0) - (getTokensPerSecondValue(rowB.original) ?? 0),
    },
    {
      id: 'caller',
      accessorFn: (row) => row.apiKey?.id ?? '',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.caller')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const request = row.original;
        if (request.source !== 'api') {
          return <Badge variant='secondary'>{t(`requests.source.${request.source}`)}</Badge>;
        }

        return <span className='font-mono text-xs'>{request.apiKey?.name || '-'}</span>;
      },
      filterFn: (row, _id, value) => value.length === 0 || value.includes(row.original.apiKey?.id ?? ''),
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} />,
      enableSorting: true,
      enableHiding: true,
      cell: ({ row }) => (
        <span className='text-xs whitespace-nowrap'>
          {format(new Date(row.original.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}
        </span>
      ),
    },
    {
      id: 'details',
      header: () => <span className='sr-only'>{t('requests.columns.details')}</span>,
      cell: ({ row }) => (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              className='h-8 w-8'
              onClick={() => openDetail(row.original.id)}
              aria-label={t('requests.actions.viewDetails')}
            >
              <FileText className='h-4 w-4' />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t('requests.actions.viewDetails')}</TooltipContent>
        </Tooltip>
      ),
      enableHiding: false,
    },
  ];

  return columns;
}
