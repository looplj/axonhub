import { IconAlertTriangle, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useMessageChannels } from '../context/message-channels-context';
import { useDeleteMessageChannel } from '../data/message-channels';

export function MessageChannelsBulkDeleteDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedMessageChannels, resetRowSelection, setSelectedMessageChannels } = useMessageChannels();
  const deleteMessageChannel = useDeleteMessageChannel();

  const isDialogOpen = open === 'delete';
  const selectedCount = selectedMessageChannels.length;

  if (selectedCount === 0 && !isDialogOpen) {
    return null;
  }

  const handleConfirm = async () => {
    try {
      const ids = selectedMessageChannels.map((channel) => channel.id);
      if (ids.length === 0) {
        return;
      }

      await Promise.all(ids.map((id) => deleteMessageChannel.mutateAsync(id)));
      resetRowSelection?.();
      setSelectedMessageChannels([]);
      setOpen(null);
    } catch (error) {
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          setOpen(null);
        } else {
          setOpen('delete');
        }
      }}
      handleConfirm={handleConfirm}
      disabled={selectedCount === 0}
      isLoading={deleteMessageChannel.isPending}
      confirmText={t('common.buttons.delete')}
      cancelBtnText={t('common.buttons.cancel')}
      title={
        <span className='text-destructive flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('messageChannels.dialogs.bulkDelete.title')}
        </span>
      }
      desc={t('messageChannels.dialogs.bulkDelete.description', { count: selectedCount })}
    >
      <div className='flex items-start gap-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm dark:border-red-900 dark:bg-red-900/20'>
        <IconTrash className='mt-0.5 h-4 w-4 text-red-600 dark:text-red-400' />
        <div className='space-y-1 text-left'>
          <p className='font-semibold text-red-900 dark:text-red-100'>{t('messageChannels.dialogs.bulkDelete.warning')}</p>
          <p className='text-red-800 dark:text-red-200'>{t('messageChannels.dialogs.bulkDelete.warningDetail')}</p>
        </div>
      </div>
    </ConfirmDialog>
  );
}
