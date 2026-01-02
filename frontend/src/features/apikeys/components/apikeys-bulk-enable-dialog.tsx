'use client';

import { IconAlertTriangle, IconCheck } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useApiKeysContext } from '../context/apikeys-context';
import { useBulkEnableApiKeys } from '../data/apikeys';

export function ApiKeysBulkEnableDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKeys, resetRowSelection, setSelectedApiKeys } = useApiKeysContext();
  const bulkEnableApiKeys = useBulkEnableApiKeys();

  if (!selectedApiKeys || selectedApiKeys.length === 0) return null;

  const handleBulkEnable = async () => {
    try {
      const ids = selectedApiKeys.map((apiKey) => apiKey.id);
      await bulkEnableApiKeys.mutateAsync(ids);
      resetRowSelection();
      setSelectedApiKeys([]);
      closeDialog();
    } catch (error) {
      console.error('Failed to bulk enable API keys:', error);
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.bulkEnable}
      onOpenChange={() => closeDialog('bulkEnable')}
      handleConfirm={handleBulkEnable}
      disabled={bulkEnableApiKeys.isPending}
      isLoading={bulkEnableApiKeys.isPending}
      title={
        <span className='text-primary flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('apikeys.dialogs.bulkEnable.title')}
        </span>
      }
      desc={t('apikeys.dialogs.bulkEnable.description', { count: selectedApiKeys.length })}
      confirmText={t('common.buttons.enable')}
      cancelBtnText={t('common.buttons.cancel')}
    >
      <div className='flex items-start gap-3 rounded-md border border-green-200 bg-green-50 p-3 text-sm dark:border-green-900 dark:bg-green-900/20'>
        <IconCheck className='mt-0.5 h-4 w-4 text-green-600 dark:text-green-400' />
        <div className='space-y-1 text-left'>
          <p>{t('apikeys.dialogs.bulkEnable.warning')}</p>
        </div>
      </div>
    </ConfirmDialog>
  );
}
