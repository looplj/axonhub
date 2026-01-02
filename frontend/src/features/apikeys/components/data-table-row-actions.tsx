import React from 'react';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { Row } from '@tanstack/react-table';
import { IconUserOff, IconUserCheck, IconEdit, IconSettings, IconArchive } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { useApiKeysContext } from '../context/apikeys-context';
import { ApiKey } from '../data/schema';

interface DataTableRowActionsProps {
  row: Row<ApiKey>;
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const { openDialog } = useApiKeysContext();
  const apiKey = row.original;
  const [open, setOpen] = React.useState(false);

  const handleEdit = (apiKey: ApiKey) => {
    setOpen(false);
    setTimeout(() => openDialog('edit', apiKey), 0);
  };

  const handleStatusChange = (apiKey: ApiKey) => {
    if (apiKey.status === 'archived') {
      // Archived API keys cannot be enabled/disabled
      return;
    }
    setOpen(false);
    setTimeout(() => openDialog('status', apiKey), 0);
  };

  const handleArchive = (apiKey: ApiKey) => {
    setOpen(false);
    setTimeout(() => openDialog('archive', apiKey), 0);
  };

  const handleProfiles = (apiKey: ApiKey) => {
    setOpen(false);
    setTimeout(() => openDialog('profiles', apiKey), 0);
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0'>
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>Open menu</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        <DropdownMenuItem onClick={() => handleEdit(apiKey)}>
          <IconEdit className='mr-2 h-4 w-4' />
          {t('common.actions.edit')}
        </DropdownMenuItem>
        {apiKey.type !== 'service_account' && (
          <DropdownMenuItem onClick={() => handleProfiles(apiKey)}>
            <IconSettings className='mr-2 h-4 w-4' />
            {t('apikeys.actions.profiles')}
          </DropdownMenuItem>
        )}
        {apiKey.status !== 'archived' && (
          <DropdownMenuItem
            onClick={() => handleStatusChange(apiKey)}
            className={apiKey.status === 'enabled' ? 'text-orange-600' : 'text-green-600'}
          >
            {apiKey.status === 'enabled' ? (
              <>
                <IconUserOff className='mr-2 h-4 w-4' />
                {t('common.buttons.disable')}
              </>
            ) : (
              <>
                <IconUserCheck className='mr-2 h-4 w-4' />
                {t('common.buttons.enable')}
              </>
            )}
          </DropdownMenuItem>
        )}
        {apiKey.status !== 'archived' && (
          <DropdownMenuItem onClick={() => handleArchive(apiKey)} className='text-orange-600'>
            <IconArchive className='mr-2 h-4 w-4' />
            {t('common.buttons.archive')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
