'use client';

import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useDataStoragesContext } from '../context/data-storages-context';
import { useArchiveDataStorage } from '../data/data-storages';

export function ArchiveDataStorageDialog() {
  const { t } = useTranslation();
  const {
    isArchiveDialogOpen,
    setIsArchiveDialogOpen,
    archiveDataStorage,
    setArchiveDataStorage,
  } = useDataStoragesContext();
  const archiveMutation = useArchiveDataStorage();

  const resetArchiveContext = () => {
    setIsArchiveDialogOpen(false);
    setArchiveDataStorage(null);
  };

  return (
    <Dialog open={isArchiveDialogOpen} onOpenChange={setIsArchiveDialogOpen}>
      <DialogContent className='sm:max-w-[480px]'>
        <DialogHeader>
          <DialogTitle>{t('dataStorages.dialogs.status.archiveTitle')}</DialogTitle>
          <DialogDescription>
            {t('dataStorages.dialogs.status.archiveDescription', {
              name: archiveDataStorage?.name ?? '',
            })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type='button' variant='outline' onClick={resetArchiveContext}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            disabled={archiveMutation.isPending}
            onClick={async () => {
              if (!archiveDataStorage) return;
              try {
                await archiveMutation.mutateAsync(archiveDataStorage.id);
                resetArchiveContext();
              } catch (_error) {
                // handled in mutation
              }
            }}
          >
            {archiveMutation.isPending ? t('common.buttons.archiving') : t('common.buttons.archive')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
