'use client';

import { Archive, MoreHorizontal, Pencil } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { useDataStoragesContext } from '../context/data-storages-context';
import { DataStorage } from '../data/data-storages';

interface DataStorageActionsProps {
  dataStorage: DataStorage;
  defaultDataStorageID?: string | null;
}

export function DataStorageActions({ dataStorage, defaultDataStorageID }: DataStorageActionsProps) {
  const { t } = useTranslation();
  const { setEditingDataStorage, setIsEditDialogOpen, setArchiveDataStorage, setIsArchiveDialogOpen } = useDataStoragesContext();
  const { isOwner } = usePermissions();

  const handleEdit = () => {
    setEditingDataStorage(dataStorage);
    setIsEditDialogOpen(true);
  };

  const handleArchive = () => {
    setArchiveDataStorage(dataStorage);
    setIsArchiveDialogOpen(true);
  };

  // Don't show menu if user is not owner
  if (!isOwner) {
    return null;
  }

  // Primary data storage cannot be edited
  if (dataStorage.primary) {
    return null;
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='h-8 w-8 p-0'>
          <span className='sr-only'>{t('common.openMenu')}</span>
          <MoreHorizontal className='h-4 w-4' />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onClick={handleEdit}>
          <Pencil className='mr-2 h-4 w-4' />
          {t('common.buttons.edit')}
        </DropdownMenuItem>
        {dataStorage.status !== 'archived' && dataStorage.id !== defaultDataStorageID && (
          <DropdownMenuItem onClick={handleArchive}>
            <Archive className='mr-2 h-4 w-4' />
            {t('common.buttons.archive')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
