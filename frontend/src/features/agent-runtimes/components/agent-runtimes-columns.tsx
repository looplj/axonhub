import { useCallback, useState, memo } from 'react';
import { format } from 'date-fns';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import {
  IconEdit,
  IconTrash,
  IconCheck,
  IconBan,
  IconServer,
  IconContainer,
} from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
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
import { useAgentRuntimes } from '../context/agent-runtimes-context';
import { useUpdateAgentRuntimeStatus } from '../data/agent-runtimes';
import { AgentRuntime, AgentRuntimeType, AgentRuntimeStatus } from '../data/schema';

// Status Switch Cell Component to handle status toggle
const StatusSwitchCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const agentRuntime = row.original;
  const updateStatus = useUpdateAgentRuntimeStatus();

  const isActive = agentRuntime.status === 'active';

  const handleSwitchChange = useCallback(async () => {
    const newStatus: AgentRuntimeStatus = isActive ? 'inactive' : 'active';
    try {
      await updateStatus.mutateAsync({
        id: agentRuntime.id,
        status: newStatus,
      });
    } catch (_error) {}
  }, [agentRuntime.id, isActive, updateStatus]);

  return (
    <div className="flex justify-center">
      <Switch
        checked={isActive}
        onCheckedChange={handleSwitchChange}
        disabled={updateStatus.isPending}
        data-testid="agent-runtime-status-switch"
      />
    </div>
  );
});

StatusSwitchCell.displayName = 'StatusSwitchCell';

// Action Cell Component
const ActionCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const { t } = useTranslation();
  const agentRuntime = row.original;
  const { setOpen, setCurrentRow } = useAgentRuntimes();
  const { agentRuntimesPermissions } = usePermissions();

  const handleEdit = useCallback(() => {
    setCurrentRow(agentRuntime);
    setOpen('edit');
  }, [agentRuntime, setCurrentRow, setOpen]);

  return (
    <div className="flex items-center justify-center gap-1">
      <Button size="sm" variant="outline" className="h-8 w-8 p-0" onClick={handleEdit}>
        <IconEdit className="h-3 w-3" />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size="sm" variant="outline" className="h-8 w-8 p-0" data-testid="row-actions">
            <DotsHorizontalIcon className="h-3 w-3" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-[160px]">
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(agentRuntime);
              setOpen('delete');
            }}
            className="text-red-500!"
          >
            <IconTrash size={16} className="mr-2" />
            {t('common.buttons.delete')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
});

ActionCell.displayName = 'ActionCell';

// Type Cell Component
const TypeCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const { t } = useTranslation();
  const type = row.original.type as AgentRuntimeType;

  const typeConfig: Record<AgentRuntimeType, { icon: React.ElementType; label: string; color: string }> = {
    vm: { icon: IconServer, label: t('agentRuntimes.types.vm'), color: 'bg-blue-100 text-blue-700 border-blue-200' },
    docker: { icon: IconContainer, label: t('agentRuntimes.types.docker'), color: 'bg-cyan-100 text-cyan-700 border-cyan-200' },
  };

  const config = typeConfig[type];
  const IconComponent = config.icon;

  return (
    <div className="flex justify-center">
      <Badge variant="outline" className={cn('capitalize', config.color)}>
        <div className="flex items-center gap-2">
          <IconComponent size={16} className="shrink-0" />
          <span>{config.label}</span>
        </div>
      </Badge>
    </div>
  );
});

TypeCell.displayName = 'TypeCell';

// Status Cell Component
const StatusCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const { t } = useTranslation();
  const status = row.original.status as AgentRuntimeStatus;

  const statusConfig: Record<AgentRuntimeStatus, { label: string; color: string }> = {
    active: { label: t('agentRuntimes.status.active'), color: 'bg-green-100 text-green-700 border-green-200' },
    inactive: { label: t('agentRuntimes.status.inactive'), color: 'bg-gray-100 text-gray-700 border-gray-200' },
    error: { label: t('agentRuntimes.status.error'), color: 'bg-red-100 text-red-700 border-red-200' },
  };

  const config = statusConfig[status];

  return (
    <div className="flex justify-center">
      <Badge variant="outline" className={cn('capitalize', config.color)}>
        {config.label}
      </Badge>
    </div>
  );
});

StatusCell.displayName = 'StatusCell';

// Name Cell Component
const NameCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const agentRuntime = row.original;
  const hasError = agentRuntime.status === 'error';

  return (
    <div className="flex justify-center">
      <div className="flex max-w-56 items-center gap-2">
        <div className={cn('truncate font-medium', hasError && 'text-destructive')}>
          {row.getValue('name')}
        </div>
      </div>
    </div>
  );
});

NameCell.displayName = 'NameCell';

// Host Cell Component
const HostCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const host = row.getValue('host') as string;

  return (
    <div className="flex justify-center">
      <code className="bg-muted rounded px-2 py-0.5 font-mono text-xs">{host || '-'}</code>
    </div>
  );
});

HostCell.displayName = 'HostCell';

// User Cell Component
const UserCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const user = row.getValue('user') as string;

  return (
    <div className="flex justify-center">
      <span className="text-sm">{user || '-'}</span>
    </div>
  );
});

UserCell.displayName = 'UserCell';

// Created At Cell Component
const CreatedAtCell = memo(({ row }: { row: Row<AgentRuntime> }) => {
  const raw = row.getValue('createdAt') as unknown;
  const date = raw instanceof Date ? raw : new Date(raw as string);

  if (Number.isNaN(date.getTime())) {
    return (
      <div className="flex justify-center">
        <span className="text-muted-foreground text-xs">-</span>
      </div>
    );
  }

  return (
    <div className="flex justify-center">
      <Tooltip>
        <TooltipTrigger asChild>
          <div className="text-muted-foreground cursor-help text-sm">{format(date, 'yyyy-MM-dd')}</div>
        </TooltipTrigger>
        <TooltipContent>{format(date, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
      </Tooltip>
    </div>
  );
});

CreatedAtCell.displayName = 'CreatedAtCell';

export const createColumns = (
  t: ReturnType<typeof useTranslation>['t'],
  canWrite: boolean = true
): ColumnDef<AgentRuntime>[] => {
  return [
    ...(canWrite
      ? [
          {
            id: 'select',
            header: ({ table }: { table: Table<AgentRuntime> }) => (
              <div className="flex justify-center">
                <Checkbox
                  checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
                  onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
                  aria-label={t('common.columns.selectAll')}
                  className="translate-y-[2px]"
                />
              </div>
            ),
            cell: ({ row }: { row: Row<AgentRuntime> }) => (
              <div className="flex justify-center">
                <Checkbox
                  checked={row.getIsSelected()}
                  onCheckedChange={(value) => row.toggleSelected(!!value)}
                  aria-label={t('common.columns.selectRow')}
                  className="translate-y-[2px]"
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
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('common.columns.name')} className="justify-center" />
      ),
      cell: NameCell,
      meta: {
        className: 'md:table-cell min-w-48 text-center',
      },
      enableHiding: false,
      enableSorting: true,
    },
    {
      accessorKey: 'type',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('agentRuntimes.columns.type')} className="justify-center" />
      ),
      cell: TypeCell,
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
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('common.columns.status')} className="justify-center" />
      ),
      cell: StatusCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'host',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('agentRuntimes.columns.host')} className="justify-center" />
      ),
      cell: HostCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: false,
    },
    {
      accessorKey: 'user',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('agentRuntimes.columns.user')} className="justify-center" />
      ),
      cell: UserCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: false,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} className="justify-center" />
      ),
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
              <DataTableColumnHeader column={column} title={t('common.columns.actions')} className="justify-center" />
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
