'use client';

import { IconAlertTriangle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useApiKeysContext } from '../context/apikeys-context';
import { useDeleteApiKey } from '../data/apikeys';

export function ApiKeysDeleteDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey, resetRowSelection } = useApiKeysContext();
  const deleteApiKey = useDeleteApiKey();

  if (!selectedApiKey) return null;

  const handleDelete = async () => {
    try {
      await deleteApiKey.mutateAsync(selectedApiKey.id);
      closeDialog('delete');
      resetRowSelection();
    } catch (_error) {
      // Error is handled by the mutation.
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.delete}
      onOpenChange={(open) => {
        if (!open) closeDialog('delete');
      }}
      handleConfirm={handleDelete}
      isLoading={deleteApiKey.isPending}
      destructive
      title={
        <span className='text-destructive'>
          <IconAlertTriangle className='stroke-destructive mr-1 inline-block' size={18} />
          {t('apikeys.dialogs.delete.title')}
        </span>
      }
      desc={
        <div className='space-y-2'>
          <p>{t('apikeys.dialogs.delete.description', { name: selectedApiKey.name })}</p>
          <p className='text-destructive font-medium'>{t('apikeys.dialogs.delete.warning')}</p>
          <p>{t('apikeys.dialogs.delete.historyRetained')}</p>
        </div>
      }
      confirmText={t('common.buttons.delete')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
