import { useCallback, useState, memo, useRef, useEffect } from 'react';
import { format } from 'date-fns';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import {
  IconPlayerPlay,
  IconChevronDown,
  IconChevronRight,
  IconAlertTriangle,
  IconEdit,
  IconArchive,
  IconTrash,
  IconCheck,
  IconWeight,
  IconTransform,
  IconNetwork,
  IconAdjustments,
  IconRoute,
  IconCopy,
  IconCoin,
  IconLoader2,
  IconKey,
  IconKeyOff,
  IconGauge,
  IconHistory,
  IconPlugConnected,
  IconClockPlay,
} from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { useChannels } from '../context/channels-context';
import { useTestChannel, useUpdateChannel } from '../data/channels';
import { CHANNEL_CONFIGS, getProvider } from '../data/config_channels';
import { Channel } from '../data/schema';
import { ChannelHealthCell } from './channel-health-cell';
import { ChannelLimiterCell } from './channel-limiter-cell';
import { ChannelsStatusDialog } from './channels-status-dialog';

const WEIGHT_PRECISION = 4;
const MIN_WEIGHT = 0;
const MAX_WEIGHT = 100;
const QUOTA_VISIBLE_LIMIT = 5;

const OAUTH_CHANNEL_TYPES = new Set<Channel['type']>(['codex', 'claudecode', 'antigravity', 'github_copilot', 'xai_subscription']);

type QuotaLimit = {
  window?: string;
  usageRatio?: number;
  status?: string;
};

function getQuotaLimits(channel: Channel): QuotaLimit[] {
  const quotaStatus = channel.providerQuotaStatus;
  if (!quotaStatus) return [];

  const data = quotaStatus.quotaData as Record<string, unknown>;
  const limits = Array.isArray(data._limits)
    ? data._limits.filter((limit): limit is Record<string, unknown> => typeof limit === 'object' && limit !== null)
    : [];
  const normalized = limits.map((limit) => ({
    window: typeof limit.window === 'string' ? limit.window : undefined,
    usageRatio: typeof limit.usageRatio === 'number' ? limit.usageRatio : undefined,
    status: typeof limit.status === 'string' ? limit.status : undefined,
  }));

  // Older persisted xAI statuses have unlabeled normalized limits. Match each
  // one to the raw billing window by its usage value instead of array position,
  // because either the weekly or monthly response may be absent.
  if (channel.type === 'xai_subscription') {
    const billing = data.billing as Record<string, unknown> | undefined;
    for (const [key, label] of [
      ['weekly', 'weekly'],
      ['monthly', 'monthly'],
    ] as const) {
      const window = billing?.[key] as Record<string, unknown> | undefined;
      if (typeof window?.usage_percent !== 'number' || normalized.some((limit) => limit.window === label)) {
        continue;
      }
      const usageRatio = window.usage_percent / 100;
      const unlabeled = normalized.find(
        (limit) => !limit.window && limit.usageRatio != null && Math.abs(limit.usageRatio - usageRatio) < 0.000001
      );
      if (unlabeled) {
        unlabeled.window = label;
      } else {
        normalized.push({ window: label, usageRatio, status: quotaStatus.status });
      }
    }
  }

  if (channel.type === 'claudecode' && normalized.length === 0) {
    const windows = data.windows as Record<string, unknown> | undefined;
    for (const label of ['5h', '7d']) {
      const window = windows?.[label] as Record<string, unknown> | undefined;
      if (typeof window?.utilization !== 'number') continue;
      normalized.push({ window: label, usageRatio: window.utilization, status: quotaStatus.status });
    }
  }

  if (channel.type === 'antigravity' && normalized.length === 0) {
    const models = data.models as Record<string, unknown> | undefined;
    for (const [modelID, value] of Object.entries(models ?? {})) {
      if (typeof value !== 'object' || value === null) continue;
      const model = value as Record<string, unknown>;
      if (typeof model.remainingPercentage !== 'number') continue;
      normalized.push({
        window: typeof model.displayName === 'string' && model.displayName ? model.displayName : modelID,
        usageRatio: 1 - model.remainingPercentage / 100,
        status: typeof model.status === 'string' ? model.status : undefined,
      });
    }
  }

  // Codex exposes a secondary window in rate_limit while its normalized
  // provider limit currently contains only the primary window.
  if (channel.type === 'codex') {
    const rateLimit = data.rate_limit as Record<string, unknown> | undefined;
    for (const [key, label] of [
      ['primary_window', '5h'],
      ['secondary_window', '7d'],
    ] as const) {
      const window = rateLimit?.[key] as Record<string, unknown> | undefined;
      if (typeof window?.used_percent !== 'number') continue;
      const alreadyIncluded = normalized.some(
        (limit) => limit.window === label || (key === 'primary_window' && limit.window === 'primary')
      );
      if (!alreadyIncluded) {
        normalized.push({
          window: label,
          usageRatio: window.used_percent / 100,
          status: quotaStatus.status,
        });
      }
    }
  }

  if (channel.type === 'antigravity') {
    normalized.sort((a, b) => (b.usageRatio ?? 0) - (a.usageRatio ?? 0));
  }

  return normalized.filter((limit) => limit.usageRatio != null || limit.status === 'exhausted');
}

function quotaWindowLabel(window: string | undefined): string {
  if (!window) return '';
  if (window === 'primary') return '5h';
  if (window === 'secondary') return '7d';
  if (window === 'daily') return '1d';
  if (window === 'weekly') return '7d';
  if (window === 'monthly') return '30d';
  return window;
}

const quotaColor = (remaining: number) => {
  if (remaining <= 20) return 'text-red-500';
  if (remaining <= 50) return 'text-yellow-500';
  return 'text-green-600 dark:text-green-500';
};

const formatWeight = (value: number) => Number(value.toFixed(WEIGHT_PRECISION));
const clampWeight = (value: number) => formatWeight(Math.min(MAX_WEIGHT, Math.max(MIN_WEIGHT, value)));

// Status Switch Cell Component to handle status toggle with confirmation dialog
const StatusSwitchCell = memo(({ row }: { row: Row<Channel> }) => {
  const channel = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);
  const { channelPermissions } = usePermissions();

  const isEnabled = channel.status === 'enabled';
  const isArchived = channel.status === 'archived';

  const handleSwitchClick = useCallback(() => {
    if (!isArchived) {
      setDialogOpen(true);
    }
  }, [isArchived]);

  if (!channelPermissions.canWrite) {
    return <Badge variant='outline'>{channel.status}</Badge>;
  }

  return (
    <div className='flex justify-center'>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} disabled={isArchived} data-testid='channel-status-switch' />
      {dialogOpen && <ChannelsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={channel} />}
    </div>
  );
});

StatusSwitchCell.displayName = 'StatusSwitchCell';

// Action Cell Component to handle hooks properly
const ActionCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const { setOpen, setCurrentRow } = useChannels();
  const { channelPermissions } = usePermissions();
  const testChannel = useTestChannel();
  const isArchived = channel.status === 'archived';
  const hasError = channel.errorMessage != null;
  const hasDisabledAPIKeys = channelPermissions.canWrite && (channel.disabledAPIKeys?.length ?? 0) > 0;

  const handleDefaultTest = async () => {
    try {
      await testChannel.mutateAsync({
        channelID: channel.id,
        modelID: channel.defaultTestModel || undefined,
      });
    } catch (_error) {}
  };

  const handleOpenTestDialog = useCallback(() => {
    setCurrentRow(channel);
    setOpen('test');
  }, [channel, setCurrentRow, setOpen]);

  const handleEdit = useCallback(() => {
    setCurrentRow(channel);
    setOpen('edit');
  }, [channel, setCurrentRow, setOpen]);

  return (
    <div className='flex items-center justify-center gap-1'>
      <Button size='sm' variant='outline' className='h-8 w-8 p-0' onClick={handleEdit}>
        <IconEdit className='h-3 w-3' />
      </Button>
      <Button size='sm' variant='outline' className='h-8 px-3' onClick={handleDefaultTest} disabled={testChannel.isPending}>
        <IconPlayerPlay className='mr-1 h-3 w-3' />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size='sm' variant='outline' className='h-8 w-8 p-0' data-testid='row-actions'>
            <DotsHorizontalIcon className='h-3 w-3' />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[160px]'>
          <DropdownMenuItem onClick={handleOpenTestDialog}>
            <IconPlayerPlay size={16} className='mr-2' />
            {t('channels.actions.test')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('testHistory');
            }}
          >
            <IconHistory size={16} className='mr-2' />
            {t('channels.actions.testHistory')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />

          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('duplicate');
            }}
          >
            <IconCopy size={16} className='mr-2' />
            {t('common.actions.duplicate')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('modelMapping');
            }}
          >
            <IconRoute size={16} className='mr-2' />
            {t('channels.dialogs.settings.modelMapping.title')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('price');
            }}
          >
            <IconCoin size={16} className='mr-2' />
            {t('channels.actions.modelPrice')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('overrides');
            }}
          >
            <IconAdjustments size={16} className='mr-2' />
            {t('channels.dialogs.settings.overrides.action')}
          </DropdownMenuItem>

          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('proxy');
            }}
          >
            <IconNetwork size={16} className='mr-2' />
            {t('channels.dialogs.proxy.action')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('transformOptions');
            }}
          >
            <IconTransform size={16} className='mr-2' />
            {t('channels.dialogs.transformOptions.action')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('rateLimit');
            }}
          >
            <IconGauge size={16} className='mr-2' />
            {t('channels.dialogs.rateLimit.action')}
          </DropdownMenuItem>
          {channel.type !== 'xai_subscription' && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('endpoints');
              }}
            >
              <IconPlugConnected size={16} className='mr-2' />
              {t('channels.endpoints.title')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('keyManagement');
              }}
            >
              <IconKey size={16} className='mr-2' />
              {t('channels.actions.keyManagement')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('availability');
              }}
            >
              <IconClockPlay size={16} className='mr-2' />
              {t('channels.dialogs.availability.action')}
            </DropdownMenuItem>
          )}
          {hasDisabledAPIKeys && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('disabledAPIKeys');
              }}
              className='text-orange-500!'
            >
              <IconKeyOff size={16} className='mr-2' />
              {t('channels.actions.disabledAPIKeys', { count: channel.disabledAPIKeys?.length ?? 0 })}
            </DropdownMenuItem>
          )}
          {hasError && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('errorResolved');
              }}
              className='text-green-600!'
            >
              <IconCheck size={16} className='mr-2' />
              {t('channels.actions.markErrorResolved')}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('archive');
            }}
            className={isArchived ? 'text-green-600!' : 'text-orange-500!'}
          >
            {isArchived ? <IconCheck size={16} className='mr-2' /> : <IconArchive size={16} className='mr-2' />}
            {t(isArchived ? 'common.buttons.restore' : 'common.buttons.archive')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('delete');
            }}
            className='text-red-500!'
          >
            <IconTrash size={16} className='mr-2' />
            {t('common.buttons.delete')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
});

ActionCell.displayName = 'ActionCell';

const ExpandCell = ({ row }: { row: any }) => (
  <div className='flex justify-center'>
    <Button
      variant='ghost'
      size='sm'
      className='h-6 w-6 p-0'
      onClick={(e) => {
        e.stopPropagation();
        row.toggleExpanded();
      }}
    >
      {row.getIsExpanded() ? <IconChevronDown className='h-4 w-4' /> : <IconChevronRight className='h-4 w-4' />}
    </Button>
  </div>
);

// ExpandCell.displayName = 'ExpandCell'; // Removed since it's not memoized now, but can keep if desired

function getChannelWebsiteURL(baseURL: string): string | null {
  try {
    const url = new URL(baseURL);
    return url.origin;
  } catch {
    return null;
  }
}

function getProxyURLSummary(proxyURL: string): { label: string; detail?: string } {
  try {
    const url = new URL(proxyURL);
    const pathname = url.pathname === '/' ? '' : url.pathname;
    return {
      label: url.host || proxyURL,
      detail: `${url.protocol}//${url.host}${pathname}`,
    };
  } catch {
    return { label: proxyURL };
  }
}

// Memoized cell components to avoid recreating on every render
const NameCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const hasError = channel.errorMessage != null;
  const disabledKeysCount = channel.disabledAPIKeys?.length ?? 0;
  const hasDisabledKeys = disabledKeysCount > 0;
  const websiteURL = getChannelWebsiteURL(channel.baseURL);

  const nameElement = websiteURL ? (
    <a
      href={websiteURL}
      target='_blank'
      rel='noopener noreferrer'
      className={cn('truncate font-medium hover:underline', hasError ? 'text-destructive' : '')}
      onClick={(e) => e.stopPropagation()}
    >
      {row.getValue('name')}
    </a>
  ) : (
    <div className={cn('truncate font-medium', hasError && 'text-destructive')}>{row.getValue('name')}</div>
  );

  // Both indicators are shown independently: a channel disabled because every
  // credential is unavailable carries an error *and* disabled credentials, and
  // hiding the key icon behind the error would lose the reason it went down.
  const content = (
    <div className='flex justify-center'>
      <div className='flex max-w-56 items-center gap-2'>
        {hasError && <IconAlertTriangle className='text-destructive h-4 w-4 shrink-0' />}
        {hasDisabledKeys && <IconKeyOff className='h-4 w-4 shrink-0 text-amber-500' />}
        {nameElement}
      </div>
    </div>
  );

  if (!hasError && !hasDisabledKeys) {
    return content;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1'>
          {hasError && (
            <p className='text-destructive text-sm'>
              {t(`channels.messages.${channel.errorMessage}`, {
                defaultValue: channel.errorMessage,
              })}
            </p>
          )}
          {hasDisabledKeys && (
            <p className='text-sm text-amber-500'>{t('channels.actions.disabledAPIKeys', { count: disabledKeysCount })}</p>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  );
});

NameCell.displayName = 'NameCell';

const ProviderCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const type = row.original.type;
  const config = CHANNEL_CONFIGS[type];
  const provider = getProvider(type);
  const IconComponent = config.icon;
  return (
    <div className='flex justify-center'>
      <Badge variant='outline' className={cn('capitalize', config.color)}>
        <div className='flex items-center gap-2'>
          <IconComponent size={16} className='shrink-0' />
          <span>{t(`channels.providers.${provider}`)}</span>
        </div>
      </Badge>
    </div>
  );
});

ProviderCell.displayName = 'ProviderCell';

const QuotaCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const [isExpanded, setIsExpanded] = useState(false);
  const channel = row.original;

  if (!OAUTH_CHANNEL_TYPES.has(channel.type)) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  if (!channel.providerQuotaStatus) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>{t('quota.label.unavailable')}</span>
      </div>
    );
  }

  const limits = getQuotaLimits(channel);
  if (limits.length === 0) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>{t('quota.label.unavailable')}</span>
      </div>
    );
  }

  const visibleLimits = isExpanded ? limits : limits.slice(0, QUOTA_VISIBLE_LIMIT);
  const hiddenCount = limits.length - QUOTA_VISIBLE_LIMIT;
  const content = (
    <div className='flex min-w-80 flex-col items-stretch gap-1.5 text-[11px]'>
      {visibleLimits.map((limit, index) => {
        const usageRatio = limit.status === 'exhausted' ? 1 : (limit.usageRatio ?? 1);
        const remaining = Math.round(Math.max(0, Math.min(100, 100 - usageRatio * 100)));
        const label = quotaWindowLabel(limit.window) || t('quota.label.quota');
        return (
          <div key={`${label}-${index}`} className='flex items-center justify-end gap-2'>
            <span className='text-muted-foreground min-w-24 whitespace-nowrap text-left'>{label}</span>
            <div className='bg-muted h-1.5 w-24 shrink-0 overflow-hidden rounded-full'>
              <div
                className={`h-full ${remaining <= 20 ? 'bg-red-500' : remaining <= 50 ? 'bg-yellow-500' : 'bg-green-500'}`}
                style={{ width: `${remaining}%` }}
              />
            </div>
            <span className={`w-8 text-right font-medium ${quotaColor(remaining)}`}>{remaining}%</span>
          </div>
        );
      })}
      {hiddenCount > 0 && (
        <button
          type='button'
          className='text-primary hover:text-primary/80 mt-0.5 self-end text-xs font-medium hover:underline'
          aria-expanded={isExpanded}
          onClick={(event) => {
            event.stopPropagation();
            setIsExpanded((expanded) => !expanded);
          }}
        >
          {isExpanded ? t('channels.quota.collapse') : t('channels.quota.expand', { count: hiddenCount })}
        </button>
      )}
    </div>
  );

  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent className='space-y-1'>
        <div className='font-medium'>{t(`quota.status.${channel.providerQuotaStatus.status}`)}</div>
        {limits.map((limit, index) => {
          const usageRatio = limit.status === 'exhausted' ? 1 : (limit.usageRatio ?? 1);
          const remaining = Math.round(Math.max(0, Math.min(100, 100 - usageRatio * 100)));
          return (
            <div key={`${limit.window}-${index}`} className='text-xs'>
              {quotaWindowLabel(limit.window) || t('quota.label.quota')}: {remaining}%
            </div>
          );
        })}
      </TooltipContent>
    </Tooltip>
  );
});

QuotaCell.displayName = 'QuotaCell';

const TagsCell = memo(({ row }: { row: Row<Channel> }) => {
  const tags = (row.getValue('tags') as string[]) || [];
  if (tags.length === 0) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }
  return (
    <div className='flex max-w-48 flex-wrap justify-center gap-1'>
      {tags.slice(0, 2).map((tag) => (
        <Badge key={tag} variant='outline' className='text-xs'>
          {tag}
        </Badge>
      ))}
      {tags.length > 2 && (
        <Badge variant='outline' className='text-xs'>
          +{tags.length - 2}
        </Badge>
      )}
    </div>
  );
});

TagsCell.displayName = 'TagsCell';

const ProxyCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const proxy = row.original.settings?.proxy;

  if (!proxy || proxy.type === 'disabled') {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  if (proxy.type === 'environment') {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>{t('channels.dialogs.proxy.types.environment')}</span>
      </div>
    );
  }

  const proxyURL = proxy.url?.trim();
  if (!proxyURL) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  const { label, detail } = getProxyURLSummary(proxyURL);
  const content = (
    <div className='flex justify-center'>
      <span className='max-w-40 truncate font-mono text-xs'>{label}</span>
    </div>
  );

  if (detail && detail !== label) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>{content}</TooltipTrigger>
        <TooltipContent>{detail}</TooltipContent>
      </Tooltip>
    );
  }

  return content;
});

ProxyCell.displayName = 'ProxyCell';

const SupportedModelsCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const models = row.getValue('supportedModels') as string[];
  const { setOpen, setCurrentRow } = useChannels();

  const handleOpenModelsDialog = useCallback(() => {
    setCurrentRow(channel);
    setOpen('viewModels');
  }, [channel, setCurrentRow, setOpen]);

  return (
    <div className='flex items-center justify-center gap-2'>
      <div className='flex flex-wrap justify-center gap-1 overflow-hidden'>
        {models.slice(0, 5).map((model) => (
          <Badge key={model} variant='secondary' className='block max-w-48 truncate text-left text-xs'>
            {model}
          </Badge>
        ))}
        {models.length > 5 && (
          <Badge
            variant='secondary'
            className='hover:bg-primary hover:text-primary-foreground cursor-pointer text-xs transition-colors'
            onClick={handleOpenModelsDialog}
            title={t('channels.actions.viewModels')}
          >
            +{models.length - 5}
          </Badge>
        )}
      </div>
    </div>
  );
});

SupportedModelsCell.displayName = 'SupportedModelsCell';

const OrderingWeightCell = memo(({ row }: { row: Row<Channel> }) => {
  const channel = row.original;
  const initialWeight = row.getValue('orderingWeight') as number | null;
  const [isEditing, setIsEditing] = useState(false);
  const [weight, setWeight] = useState<string>(initialWeight?.toString() || '1');
  const updateChannel = useUpdateChannel();
  const { channelPermissions } = usePermissions();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleDoubleClick = useCallback(() => {
    setIsEditing(true);
    setWeight(initialWeight?.toString() || '1');
  }, [initialWeight]);

  const handleSave = useCallback(async () => {
    const weightValue = clampWeight(Number(weight));
    if (weightValue === initialWeight) {
      setIsEditing(false);
      return;
    }

    try {
      await updateChannel.mutateAsync({
        id: channel.id,
        input: { orderingWeight: weightValue },
      });
      setIsEditing(false);
    } catch (_error) {
      // Error handled by mutation hook
    }
  }, [channel.id, weight, initialWeight, updateChannel]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        handleSave();
      } else if (e.key === 'Escape') {
        setIsEditing(false);
        setWeight(initialWeight?.toString() || '1');
      }
    },
    [handleSave, initialWeight]
  );

  if (!channelPermissions.canWrite) {
    return <span className={cn('font-mono text-sm', initialWeight == null && 'text-muted-foreground')}>{initialWeight ?? '-'}</span>;
  }

  if (isEditing) {
    return (
      <div className='flex justify-center px-2'>
        <Input
          ref={inputRef}
          type='number'
          inputMode='decimal'
          step='any'
          min={MIN_WEIGHT}
          max={MAX_WEIGHT}
          value={weight}
          onChange={(e) => setWeight(e.target.value)}
          onBlur={handleSave}
          onKeyDown={handleKeyDown}
          className='h-7 w-20 text-center font-mono text-sm'
          disabled={updateChannel.isPending}
        />
      </div>
    );
  }

  return (
    <div className='flex items-center justify-center gap-2 group cursor-pointer' onDoubleClick={handleDoubleClick}>
      <span className={cn('font-mono text-sm', initialWeight == null && 'text-muted-foreground')}>
        {initialWeight ?? '-'}
      </span>
      {updateChannel.isPending && <IconLoader2 className='h-3 w-3 animate-spin text-muted-foreground' />}
    </div>
  );
});

OrderingWeightCell.displayName = 'OrderingWeightCell';

const CreatedAtCell = memo(({ row }: { row: Row<Channel> }) => {
  const raw = row.getValue('createdAt') as unknown;
  const date = raw instanceof Date ? raw : new Date(raw as string);

  if (Number.isNaN(date.getTime())) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  return (
    <div className='flex justify-center'>
      <Tooltip>
        <TooltipTrigger asChild>
          <div className='text-muted-foreground cursor-help text-sm'>{format(date, 'yyyy-MM-dd')}</div>
        </TooltipTrigger>
        <TooltipContent>{format(date, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
      </Tooltip>
    </div>
  );
});

CreatedAtCell.displayName = 'CreatedAtCell';

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = true): ColumnDef<Channel>[] => {
  return [
    {
      id: 'expand',
      header: () => null,
      meta: {
        className: 'w-8 min-w-8 text-center',
      },
      cell: ExpandCell,
      enableSorting: false,
      enableHiding: false,
    },
    ...(canWrite
      ? [
          {
            id: 'select',
            header: ({ table }: { table: Table<Channel> }) => (
              <div className='flex justify-center'>
                <Checkbox
                  checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
                  onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
                  aria-label={t('common.columns.selectAll')}
                  className='translate-y-[2px]'
                />
              </div>
            ),
            cell: ({ row }: { row: Row<Channel> }) => (
              <div className='flex justify-center'>
                <Checkbox
                  checked={row.getIsSelected()}
                  onCheckedChange={(value) => row.toggleSelected(!!value)}
                  aria-label={t('common.columns.selectRow')}
                  className='translate-y-[2px]'
                />
              </div>
            ),
            meta: {
              className: 'text-center',
            },
            enableSorting: false,
            enableHiding: false,
          },
        ]
      : []),
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.name')} className='justify-center' />,
      cell: NameCell,
      meta: {
        className: 'md:table-cell min-w-48 text-center',
      },
      enableHiding: false,
      enableSorting: true,
    },
    {
      id: 'provider',
      accessorKey: 'type',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.provider')} className='justify-center' />,
      cell: ProviderCell,
      meta: {
        className: 'text-center',
      },
      filterFn: (row, _id, value) => {
        return value.includes(row.original.type);
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.status')} className='justify-center' />,
      cell: StatusSwitchCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      id: 'quota',
      accessorFn: (row) => row.providerQuotaStatus?.status ?? '',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.quota')} className='justify-center' />,
      cell: QuotaCell,
      meta: {
        className: 'w-96 min-w-96 text-center',
      },
      enableSorting: false,
      enableHiding: true,
    },

    {
      accessorKey: 'tags',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.tags')} className='justify-center' />,
      cell: TagsCell,
      meta: {
        className: 'text-center',
      },
      filterFn: (row, id, value) => {
        const tags = (row.getValue(id) as string[]) || [];
        // Single select: value is a string, not an array
        return tags.includes(value as string);
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'model',
      accessorFn: () => '', // Virtual column for filtering only
      header: () => null,
      cell: () => null,
      filterFn: () => true, // Server-side filtering, always return true
      enableSorting: false,
      enableHiding: true,
      enableColumnFilter: false,
      enableGlobalFilter: false,
    },
    {
      accessorKey: 'supportedModels',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('channels.columns.supportedModels')} className='justify-center' />
      ),
      cell: SupportedModelsCell,
      meta: {
        className: 'max-w-64 text-center',
      },
      enableSorting: false,
    },
    {
      id: 'proxy',
      accessorFn: (row) => row.settings?.proxy?.url ?? row.settings?.proxy?.type ?? '',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.proxy')} className='justify-center' />,
      cell: ProxyCell,
      meta: {
        className: 'w-32 min-w-32 text-center',
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'health',
      accessorKey: 'health',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.health')} className='justify-center' />,
      cell: ({ row }: { row: Row<Channel> }) => {
        const probePoints = (row.original as any).probePoints || [];
        const limiterStats = row.original.liveLimiterStats;
        return (
          <div className='flex flex-col items-center gap-1'>
            <ChannelHealthCell points={probePoints} />
            {limiterStats ? <ChannelLimiterCell stats={limiterStats} /> : null}
          </div>
        );
      },
      meta: {
        className: 'text-center',
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      accessorKey: 'orderingWeight',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('channels.columns.orderingWeight')} className='justify-center' />
      ),
      cell: OrderingWeightCell,
      meta: {
        className: 'w-20 min-w-20 text-center',
      },
      sortingFn: 'alphanumeric',
      enableSorting: true,
      enableHiding: true,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} className='justify-center' />,
      cell: CreatedAtCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: true,
      enableHiding: false,
    },
    ...(canWrite
      ? [
          {
            id: 'action',
            header: ({ column }: { column: any }) => (
              <DataTableColumnHeader column={column} title={t('common.columns.actions')} className='justify-center' />
            ),
            cell: ActionCell,
            meta: {
              className: 'text-center',
            },
            enableSorting: false,
            enableHiding: false,
          },
        ]
      : []),
  ];
};
