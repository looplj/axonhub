'use client';

import { IconAlertTriangle, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useChannels } from '../context/channels-context';
import { useBulkDeleteChannels } from '../data/channels';

export function ChannelsBulkDeleteDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedChannels, resetRowSelection, setSelectedChannels } = useChannels();
  const bulkDeleteChannels = useBulkDeleteChannels();

  const isDialogOpen = open === 'bulkDelete';
  const selectedCount = selectedChannels.length;

  if (selectedCount === 0 && !isDialogOpen) {
    return null;
  }

  const handleConfirm = async () => {
    try {
      const ids = selectedChannels.map((channel) => channel.id);
      if (ids.length === 0) {
        return;
      }

      await bulkDeleteChannels.mutateAsync(ids);
      resetRowSelection();
      setSelectedChannels([]);
      setOpen(null);
    } catch (error) {
      console.error('Failed to bulk delete channels:', error);
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          setOpen(null);
        } else {
          setOpen('bulkDelete');
        }
      }}
      handleConfirm={handleConfirm}
      disabled={selectedCount === 0}
      isLoading={bulkDeleteChannels.isPending}
      confirmText={t('common.buttons.delete')}
      cancelBtnText={t('common.buttons.cancel')}
      title={
        <span className='text-destructive flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('channels.dialogs.bulkDelete.title')}
        </span>
      }
      desc={t('channels.dialogs.bulkDelete.description', { count: selectedCount })}
    >
      <div className='flex items-start gap-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm dark:border-red-900 dark:bg-red-900/20'>
        <IconTrash className='mt-0.5 h-4 w-4 text-red-600 dark:text-red-400' />
        <div className='space-y-1 text-left'>
          <p className='font-semibold text-red-900 dark:text-red-100'>{t('channels.dialogs.bulkDelete.warning')}</p>
          <p className='text-red-800 dark:text-red-200'>{t('channels.dialogs.bulkDelete.warningDetail')}</p>
        </div>
      </div>
    </ConfirmDialog>
  );
}
