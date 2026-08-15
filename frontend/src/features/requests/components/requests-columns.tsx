'use client';

import { useState } from 'react';
import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { IconArrowsJoin2, IconRoute, IconScanEye } from '@tabler/icons-react';
import { zhCN, enUS } from 'date-fns/locale';
import { Ban, FileText } from 'lucide-react';
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
import { type Request } from '../data';
import { aggregateUsageByPurposeConnection, aggregateUsageConnection, type UsageSummary } from '../data/usage-summary';
import { executionDurationMs, sumExecutionDurations } from '../utils/execution-duration';
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

function getRequestExecutions(request: Request) {
  const executions =
    request.executions?.edges
      ?.map((edge) => edge.node)
      .filter((execution): execution is NonNullable<typeof execution> => Boolean(execution)) ?? [];
  const visionExecutions =
    request.visionExecutions?.edges
      ?.map((edge) => edge.node)
      .filter((execution): execution is NonNullable<typeof execution> => Boolean(execution)) ?? [];

  if (visionExecutions.length === 0) return executions;

  const executionIDs = new Set(executions.map((execution) => execution.id).filter(Boolean));
  return [...executions, ...visionExecutions.filter((execution) => !execution.id || !executionIDs.has(execution.id))];
}

function getVisionDelegationExecutions(request: Request) {
  const dedicatedExecutions =
    request.visionExecutions?.edges
      ?.map((edge) => edge.node)
      .filter((execution): execution is NonNullable<typeof execution> => Boolean(execution)) ?? [];

  if (request.visionExecutions) return dedicatedExecutions;
  return getRequestExecutions(request).filter((execution) => execution.purpose === 'vision_delegation');
}

function getRequestUsageByPurpose(request: Request) {
  const fallback = aggregateUsageByPurposeConnection(request.usageLogs);

  return {
    primary: aggregateUsageConnection(request.primaryUsageLogs) ?? fallback.primary,
    visionDelegation: aggregateUsageConnection(request.visionUsageLogs) ?? fallback.visionDelegation,
  };
}

function getPrimaryRequestUsage(request: Request) {
  return getRequestUsageByPurpose(request).primary;
}

function getRequestTotalCost(request: Request) {
  const { primary, visionDelegation } = getRequestUsageByPurpose(request);
  const costs = [primary?.totalCost, visionDelegation?.totalCost].filter((cost): cost is number => cost != null);

  return costs.length > 0 ? costs.reduce((total, cost) => total + cost, 0) : null;
}

function VisionDelegationModelIndicator({ modelIDs, usage }: { modelIDs: string[]; usage: UsageSummary | null }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const hasVisionDelegation = modelIDs.length > 0 || usage != null;
  const modelLabel = modelIDs.join(', ') || t('requests.columns.unknown');
  const cacheHitRate = usage && usage.promptTokens > 0 ? (usage.promptCachedTokens / usage.promptTokens) * 100 : 0;
  const writeCacheRate = usage && usage.promptTokens > 0 ? (usage.promptWriteCachedTokens / usage.promptTokens) * 100 : 0;
  const label = hasVisionDelegation
    ? `${t('requests.executionPurpose.visionDelegation')}: ${modelLabel}`
    : t('requests.tooltips.visionDelegationNotApplied');

  return (
    <Tooltip open={open} onOpenChange={setOpen}>
      <TooltipTrigger asChild>
        <button
          type='button'
          className={`inline-flex h-5 w-5 shrink-0 items-center justify-center rounded transition-colors ${
            hasVisionDelegation
              ? 'text-sky-600 hover:bg-sky-100 hover:text-sky-700 dark:text-sky-400 dark:hover:bg-sky-950/50 dark:hover:text-sky-300'
              : 'text-muted-foreground/45 hover:bg-muted hover:text-muted-foreground'
          }`}
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
          aria-label={label}
        >
          <IconScanEye className='h-3.5 w-3.5' />
        </button>
      </TooltipTrigger>
      <TooltipContent
        side='right'
        sideOffset={6}
        className='border-border bg-popover text-popover-foreground [&>svg]:bg-popover! [&>svg]:fill-popover! border p-0 shadow-lg'
      >
        {hasVisionDelegation ? (
          <div className='min-w-56 overflow-hidden rounded-md'>
            <div className='border-border flex items-center gap-2 border-b px-3 py-2'>
              <IconScanEye className='h-4 w-4 shrink-0 text-sky-600 dark:text-sky-400' />
              <div className='min-w-0'>
                <div className='text-muted-foreground text-[11px]'>{t('requests.executionPurpose.visionDelegation')}</div>
                <div className='truncate font-mono text-xs font-semibold'>{modelLabel}</div>
              </div>
            </div>
            <div className='space-y-1.5 px-3 py-2 font-mono text-xs'>
              <div className='flex items-center justify-between gap-6'>
                <span className='text-muted-foreground font-sans'>{t('usageLogs.columns.totalTokens')}</span>
                <span className='font-medium'>{usage ? usage.totalTokens.toLocaleString() : '-'}</span>
              </div>
              <div className='flex items-center justify-between gap-6'>
                <span className='text-muted-foreground font-sans'>{t('requests.columns.input')}</span>
                <span>{usage ? usage.promptTokens.toLocaleString() : '-'}</span>
              </div>
              <div className='flex items-center justify-between gap-6'>
                <span className='text-muted-foreground font-sans'>{t('requests.columns.output')}</span>
                <span>{usage ? usage.completionTokens.toLocaleString() : '-'}</span>
              </div>
              <div className='border-border my-2 border-t' />
              <div className='flex items-center justify-between gap-6'>
                <span className='text-muted-foreground font-sans'>{t('requests.columns.readCache')}</span>
                <span>
                  {usage ? usage.promptCachedTokens.toLocaleString() : '-'}
                  {usage && (
                    <span className='text-muted-foreground ml-1 font-sans'>
                      ({t('requests.columns.cacheHitRate', { rate: cacheHitRate.toFixed(1) })})
                    </span>
                  )}
                </span>
              </div>
              <div className='flex items-center justify-between gap-6'>
                <span className='text-muted-foreground font-sans'>{t('requests.columns.writeCache')}</span>
                <span>
                  {usage ? usage.promptWriteCachedTokens.toLocaleString() : '-'}
                  {usage && (
                    <span className='text-muted-foreground ml-1 font-sans'>
                      ({t('requests.columns.writeCacheRate', { rate: writeCacheRate.toFixed(1) })})
                    </span>
                  )}
                </span>
              </div>
            </div>
          </div>
        ) : (
          <div className='flex items-center gap-2.5 px-3 py-2'>
            <span className='bg-muted text-muted-foreground inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md'>
              <IconScanEye className='h-3.5 w-3.5' />
            </span>
            <div className='space-y-0.5'>
              <div className='text-xs font-medium'>{t('requests.executionPurpose.visionDelegation')}</div>
              <div className='text-muted-foreground text-[11px] whitespace-nowrap'>
                {t('requests.tooltips.visionDelegationNotUsedDetail')}
              </div>
            </div>
          </div>
        )}
      </TooltipContent>
    </Tooltip>
  );
}

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
        const allExecutions = getRequestExecutions(request);
        const executions = allExecutions.filter((execution) => execution.purpose !== 'vision_delegation');
        const visionDelegationModelIDs = Array.from(
          new Set(
            getVisionDelegationExecutions(request)
              .map((execution) => execution.modelID || '')
              .filter(Boolean)
          )
        );
        const { visionDelegation: visionDelegationUsage } = getRequestUsageByPurpose(request);
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
            <div className='flex items-center gap-1.5'>{modelLabel}</div>
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
                <TooltipContent>
                  {t(passThroughApplied ? 'requests.tooltips.passThroughApplied' : 'requests.tooltips.passThroughNotApplied')}
                </TooltipContent>
              </Tooltip>
              <VisionDelegationModelIndicator modelIDs={visionDelegationModelIDs} usage={visionDelegationUsage} />
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
            accessorFn: (row) => {
              const executions = getRequestExecutions(row);
              return (
                executions.find((execution) => execution.purpose !== 'vision_delegation')?.channel?.id ??
                row.channel?.id ??
                executions.find((execution) => execution.channel)?.channel?.id ??
                ''
              );
            },
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.channel')} />,
            enableSorting: false,
            enableHiding: true,
            cell: ({ row }) => {
              const request = row.original;
              const executions = getRequestExecutions(request);
              const primaryExecution = executions.find((execution) => execution.purpose !== 'vision_delegation');
              const channel = primaryExecution?.channel ?? request.channel ?? executions.find((execution) => execution.channel)?.channel;

              if (!channel) return <span className='text-muted-foreground font-mono text-xs'>-</span>;

              if (executions.length === 0) return <span className='font-mono text-xs'>{channel.name}</span>;

              const sortedExecutions = [...executions].sort((a, b) => {
                const dateA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
                const dateB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
                return dateA - dateB;
              });
              const totalCount = request.executions?.totalCount ?? executions.length;

              return (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span
                      tabIndex={0}
                      className='border-border bg-muted/40 hover:bg-muted text-foreground flex w-fit cursor-help items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium transition-colors'
                      aria-label={t('requests.tooltips.upstreamCalls', { count: totalCount })}
                    >
                      <span>{channel.name}</span>
                      {totalCount > 1 && (
                        <span className='text-primary inline-flex items-center gap-0.5'>
                          <IconArrowsJoin2 className='h-3.5 w-3.5' />
                          {totalCount}
                        </span>
                      )}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent
                    side='right'
                    className='border-border bg-popover text-popover-foreground [&>svg]:bg-popover [&>svg]:fill-popover p-0 shadow-md'
                  >
                    <div className='flex min-w-[260px] flex-col gap-1 p-2'>
                      {sortedExecutions.map((execution, index) => (
                        <div
                          key={execution.id || index}
                          className='hover:bg-muted flex items-center gap-2 rounded px-2 py-1.5 transition-colors'
                        >
                          <Badge
                            className={`${getStatusColor(execution.status || '')} h-5 shrink-0 px-1.5 text-[10px] font-bold uppercase`}
                          >
                            {execution.status ? t(`requests.status.${execution.status}`) : t('requests.columns.unknown')}
                          </Badge>
                          <div className='flex min-w-0 flex-1 flex-col'>
                            <span className='truncate text-xs font-semibold'>{execution.modelID || t('requests.columns.unknown')}</span>
                            <span className='text-muted-foreground truncate text-[10px]'>
                              {execution.purpose === 'vision_delegation'
                                ? t('requests.executionPurpose.visionDelegation')
                                : t('requests.executionPurpose.primary')}
                              {execution.channel?.name ? ` · ${execution.channel.name}` : ''}
                              {execution.createdAt ? ` · ${format(new Date(execution.createdAt), 'HH:mm:ss', { locale })}` : ''}
                            </span>
                          </div>
                        </div>
                      ))}
                      {totalCount > executions.length && (
                        <span className='text-muted-foreground px-2 text-[10px]'>
                          {t('requests.columns.moreExecutions', { count: totalCount - executions.length })}
                        </span>
                      )}
                    </div>
                  </TooltipContent>
                </Tooltip>
              );
            },
            filterFn: (row, _id, value) => {
              if (value.length === 0) return true;
              const executions = getRequestExecutions(row.original);
              const channel =
                executions.find((execution) => execution.purpose !== 'vision_delegation')?.channel ??
                row.original.channel ??
                executions.find((execution) => execution.channel)?.channel;
              return !!channel?.id && value.includes(channel.id);
            },
          },
        ] as ColumnDef<Request>[])
      : []),
    {
      id: 'tokens',
      accessorFn: (row) => getPrimaryRequestUsage(row)?.totalTokens ?? 0,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.tokens')} />,
      cell: ({ row }) => {
        const usage = getPrimaryRequestUsage(row.original);

        return usage ? (
          <div className='space-y-0.5 text-xs'>
            <div className='text-sm font-medium'>
              {t('requests.columns.totalTokens')}
              {usage.totalTokens.toLocaleString()}
            </div>
            <div className='text-muted-foreground'>
              {t('requests.columns.input')}: {usage.promptTokens.toLocaleString()} | {t('requests.columns.output')}:{' '}
              {usage.completionTokens.toLocaleString()}
            </div>
            {usage.completionReasoningTokens > 0 && (
              <div className='text-muted-foreground'>
                {t('requests.columns.reasoning')}: {usage.completionReasoningTokens.toLocaleString()}
              </div>
            )}
          </div>
        ) : (
          <div className='text-muted-foreground text-xs'>-</div>
        );
      },
      enableSorting: true,
      enableHiding: true,
      sortingFn: (rowA, rowB) =>
        (getPrimaryRequestUsage(rowA.original)?.totalTokens ?? 0) - (getPrimaryRequestUsage(rowB.original)?.totalTokens ?? 0),
    },
    {
      id: 'readCache',
      accessorFn: (row) => getPrimaryRequestUsage(row)?.promptCachedTokens ?? 0,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.readCache')} />,
      cell: ({ row }) => {
        const usage = getPrimaryRequestUsage(row.original);
        const cachedTokens = usage?.promptCachedTokens ?? 0;
        const promptTokens = usage?.promptTokens ?? 0;
        const hitRate = promptTokens > 0 ? (cachedTokens / promptTokens) * 100 : 0;
        const isLowHitRate = hitRate < 80 && promptTokens >= 40000;

        return usage && cachedTokens > 0 ? (
          <div className='text-xs'>
            <div className='text-sm font-medium'>{cachedTokens.toLocaleString()}</div>
            <div className={isLowHitRate ? 'font-medium text-red-600 dark:text-red-400' : 'text-muted-foreground'}>
              {t('requests.columns.cacheHitRate', {
                rate: hitRate.toFixed(1),
              })}
            </div>
          </div>
        ) : (
          <div className='text-muted-foreground text-xs'>-</div>
        );
      },
      enableSorting: true,
      enableHiding: true,
      sortingFn: (rowA, rowB) => {
        const a = getPrimaryRequestUsage(rowA.original)?.promptCachedTokens ?? 0;
        const b = getPrimaryRequestUsage(rowB.original)?.promptCachedTokens ?? 0;
        return a - b;
      },
    },
    {
      id: 'writeCache',
      accessorFn: (row) => getPrimaryRequestUsage(row)?.promptWriteCachedTokens ?? 0,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.writeCache')} />,
      cell: ({ row }) => {
        const usage = getPrimaryRequestUsage(row.original);

        if (!usage) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        const writeCachedTokens = usage.promptWriteCachedTokens;
        const promptTokens = usage.promptTokens;

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
        const a = getPrimaryRequestUsage(rowA.original)?.promptWriteCachedTokens ?? 0;
        const b = getPrimaryRequestUsage(rowB.original)?.promptWriteCachedTokens ?? 0;
        return a - b;
      },
    },
    {
      id: 'cost',
      accessorFn: getRequestTotalCost,
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('requests.columns.cost')} className='w-full justify-center text-center' />
      ),
      enableSorting: false,
      enableHiding: true,
      meta: {
        className: 'w-28 min-w-28 max-w-28 text-center',
      },
      cell: ({ row }) => {
        const { primary, visionDelegation } = getRequestUsageByPurpose(row.original);
        if (!primary && !visionDelegation) return <div className='text-center font-mono text-xs'>-</div>;

        const formatCost = (cost: number | null | undefined) =>
          cost == null
            ? '-'
            : t('currencies.format', {
                val: cost,
                currency: settings?.currencyCode ?? 'USD',
                locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                minimumFractionDigits: 6,
              });

        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div
                tabIndex={0}
                className='flex w-full cursor-help flex-col items-center space-y-0.5 text-center font-mono text-xs'
                aria-label={t('requests.columns.cost')}
              >
                <span className='font-medium'>{formatCost(primary?.totalCost)}</span>
                {visionDelegation && <span className='font-medium'>{formatCost(visionDelegation.totalCost)}</span>}
              </div>
            </TooltipTrigger>
            <TooltipContent>
              <div className='space-y-1 font-mono text-xs'>
                <div className='flex items-center justify-between gap-3'>
                  <span>{t('requests.cost.original')}</span>
                  <span>{formatCost(primary?.totalCost)}</span>
                </div>
                {visionDelegation && (
                  <div className='flex items-center justify-between gap-3'>
                    <span>{t('requests.cost.vision')}</span>
                    <span>{formatCost(visionDelegation.totalCost)}</span>
                  </div>
                )}
              </div>
            </TooltipContent>
          </Tooltip>
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
        const visionExecutions = getVisionDelegationExecutions(request);
        const { visionDelegation: visionUsage } = getRequestUsageByPurpose(request);
        const hasVisionDelegation = visionExecutions.length > 0 || visionUsage != null;
        const visionDurationMs = sumExecutionDurations(visionExecutions);
        const firstTokenDurationMs = request.stream ? request.metricsFirstTokenLatencyMs : null;
        const requestDurationMs =
          request.metricsLatencyMs ??
          (request.status === 'completed' || request.status === 'failed' || request.status === 'canceled'
            ? executionDurationMs(request)
            : null);

        const formatOptionalDuration = (duration: number | null | undefined) => (duration == null ? '-' : formatDuration(duration));

        return (
          <div className='min-w-[168px] space-y-0.5 font-mono text-xs'>
            <div>{t('requests.duration.firstToken', { duration: formatOptionalDuration(firstTokenDurationMs) })}</div>
            <div>{t('requests.duration.total', { duration: formatOptionalDuration(requestDurationMs) })}</div>
            {hasVisionDelegation && <div>{t('requests.duration.vision', { duration: formatOptionalDuration(visionDurationMs) })}</div>}
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
        <span className='text-xs whitespace-nowrap'>{format(new Date(row.original.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}</span>
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
