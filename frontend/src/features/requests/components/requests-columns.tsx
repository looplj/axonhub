'use client';

import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { IconArrowsJoin2, IconRoute } from '@tabler/icons-react';
import { ArrowDown, ArrowUp, Ban, FileText } from 'lucide-react';
import { zhCN, enUS } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
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
  onViewDetail?: (requestId: string) => void;
}

export const DEFAULT_HIDDEN_COLUMN_IDS = ['status', 'source', 'apiFormat', 'clientIP', 'tokensPerSecond'];

export const DEFAULT_MOBILE_HIDDEN_COLUMN_IDS = [
  ...DEFAULT_HIDDEN_COLUMN_IDS,
  'channel',
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
      id: 'request',
      accessorFn: (row) => row.createdAt,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.request')} />,
      enableSorting: true,
      enableHiding: false,
      cell: ({ row }) => {
        const request = row.original;
        return (
          <div className='flex min-w-[142px] flex-col gap-1'>
            <span className='text-sm font-medium'>{format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}</span>
            <Badge className={`${getStatusColor(request.status)} w-fit`}>{t(`requests.status.${request.status}`)}</Badge>
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
        const executions = request.executions?.edges?.flatMap((edge) => (edge.node ? [edge.node] : [])) ?? [];
        const reasoningEffort = executions[0]?.reasoningEffort ?? request.reasoningEffort;
        const passThroughApplied = executions.some((execution) => execution.passThroughApplied);

        return (
          <div className='flex min-w-[160px] flex-col gap-1'>
            <span className='font-mono text-xs font-medium'>{request.modelID || t('requests.columns.unknown')}</span>
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
              const hasExecutionPath =
                executions.length > 1 ||
                executions.some((execution) => execution.modelID && execution.modelID !== request.modelID) ||
                executions.some((execution) => execution.channel?.id && execution.channel.id !== channel?.id);

              if (!channel) return <span className='text-muted-foreground font-mono text-xs'>-</span>;

              return (
                <div className='flex min-w-[120px] items-center gap-1.5'>
                  <span className='font-mono text-xs'>{channel.name}</span>
                  {hasExecutionPath && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon-sm'
                          className='h-6 w-6 shrink-0 text-rose-600 hover:bg-rose-50 hover:text-rose-700 dark:text-rose-300 dark:hover:bg-rose-950/30 dark:hover:text-rose-200'
                          onClick={() => openDetail(request.id)}
                          aria-label={t('requests.tooltips.executionChain')}
                        >
                          <IconArrowsJoin2 className='h-3.5 w-3.5' />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent side='right' className='max-w-xs p-2'>
                        <div className='space-y-1.5'>
                          <p className='text-xs font-medium'>{t('requests.tooltips.executionChain')}</p>
                          {[...executions].reverse().map((execution, index) => (
                            <div key={execution.id ?? index} className='flex items-center gap-2 text-xs'>
                              <Badge className={`${getStatusColor(execution.status ?? '')} h-5 px-1.5 text-[10px]`}>
                                {execution.status ? t(`requests.status.${execution.status}`) : t('requests.columns.unknown')}
                              </Badge>
                              <span>{execution.channel?.name || t('requests.columns.unknown')}</span>
                            </div>
                          ))}
                        </div>
                      </TooltipContent>
                    </Tooltip>
                  )}
                </div>
              );
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
      id: 'usage',
      accessorFn: (row) => {
        const usageLog = row.usageLogs?.edges?.[0]?.node;
        return (usageLog?.promptTokens ?? 0) + (usageLog?.completionTokens ?? 0);
      },
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.usage')} />,
      enableSorting: true,
      enableHiding: false,
      cell: ({ row }) => {
        const usageLog = row.original.usageLogs?.edges?.[0]?.node;
        if (!usageLog) return <span className='text-muted-foreground text-xs'>-</span>;

        const promptTokens = usageLog.promptTokens ?? 0;
        const completionTokens = usageLog.completionTokens ?? 0;
        const readCacheTokens = usageLog.promptCachedTokens ?? 0;
        const writeCacheTokens = usageLog.promptWriteCachedTokens ?? 0;
        const hasCache = readCacheTokens > 0 || writeCacheTokens > 0;

        return (
          <div className='min-w-[170px] space-y-1 text-xs'>
            <div className='flex items-center gap-3 font-medium'>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className='inline-flex items-center gap-1' tabIndex={0} role='img' aria-label={t('requests.tooltips.inputTokens')}>
                    <ArrowUp className='h-3.5 w-3.5 text-muted-foreground' />
                    {promptTokens.toLocaleString()}
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t('requests.tooltips.inputTokens')}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className='inline-flex items-center gap-1' tabIndex={0} role='img' aria-label={t('requests.tooltips.outputTokens')}>
                    <ArrowDown className='h-3.5 w-3.5 text-muted-foreground' />
                    {completionTokens.toLocaleString()}
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t('requests.tooltips.outputTokens')}</TooltipContent>
              </Tooltip>
            </div>
            <div className='text-muted-foreground whitespace-nowrap'>
              {hasCache
                ? `${t('requests.columns.cache')} ${readCacheTokens.toLocaleString()} (${t('requests.columns.read')})  ${writeCacheTokens.toLocaleString()} (${t('requests.columns.write')})`
                : `${t('requests.columns.cache')} -`}
            </div>
          </div>
        );
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
          return <span className='font-mono text-xs'>{t('requests.duration.total', { duration: formatDuration(request.metricsLatencyMs) })} · {t('requests.stream.nonStreaming')}</span>;
        }

        return (
          <div className='min-w-[128px] font-mono text-xs'>
            {request.metricsFirstTokenLatencyMs != null && <div>{t('requests.duration.firstToken', { duration: formatDuration(request.metricsFirstTokenLatencyMs) })}</div>}
            <div className='text-muted-foreground'>{t('requests.duration.total', { duration: formatDuration(request.metricsLatencyMs) })} · {t('requests.stream.streaming')}</div>
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
