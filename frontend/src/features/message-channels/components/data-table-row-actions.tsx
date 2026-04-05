import { useTranslation } from 'react-i18next';
import { IconEdit, IconTrash, IconLink } from '@tabler/icons-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useMessageChannels } from '../context/message-channels-context';
import type { MessageChannel } from '../data/schema';

interface DataTableRowActionsProps {
  row: { original: MessageChannel };
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const { setOpen, setCurrentRow } = useMessageChannels();
  const channel = row.original;

  const handleEdit = () => {
    setCurrentRow(channel);
    setOpen('edit');
  };

  const handleDelete = () => {
    setCurrentRow(channel);
    setOpen('delete');
  };

  const handleManageBindings = () => {
    setCurrentRow(channel);
    setOpen('manageBindings');
  };

  const bindingCount = channel.agentInstanceBindings?.edges?.length || 0;

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
          <IconEdit className='h-4 w-4' />
          <span className='sr-only'>{t('common.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        <DropdownMenuItem onClick={handleEdit}>
          <IconEdit className='mr-2 h-4 w-4' />
          {t('common.actions.edit')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleManageBindings}>
          <IconLink className='mr-2 h-4 w-4' />
          {bindingCount > 0 
            ? t('messageChannels.actions.manageBindings', { count: bindingCount })
            : t('messageChannels.actions.bindAgents')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleDelete} className='text-destructive focus:text-destructive'>
          <IconTrash className='mr-2 h-4 w-4' />
          {t('common.actions.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
