'use client';

import { IconArchive, IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useApiKeysContext } from '../context/apikeys-context';
import { useUpdateApiKeyStatus } from '../data/apikeys';

export function ApiKeysArchiveDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey, resetRowSelection } = useApiKeysContext();
  const updateApiKeyStatus = useUpdateApiKeyStatus();

  if (!selectedApiKey) return null;

  const handleArchive = async () => {
    try {
      await updateApiKeyStatus.mutateAsync({
        id: selectedApiKey.id,
        status: 'archived',
      });
      closeDialog('archive');
      resetRowSelection(); // 清空选中的行
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  };

  const getDescription = () => {
    const baseDescription = t('apikeys.dialogs.archive.description', { name: selectedApiKey.name });
    const warningText = t('apikeys.dialogs.archive.warning');

    return (
      <div className='space-y-3'>
        <p>{baseDescription}</p>
        <div className='rounded-md border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-900/20'>
          <div className='flex items-start space-x-2'>
            <IconInfoCircle className='mt-0.5 h-4 w-4 flex-shrink-0 text-blue-600 dark:text-blue-400' />
            <div className='text-sm text-blue-800 dark:text-blue-200'>
              <p>{warningText}</p>
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.archive}
      onOpenChange={() => closeDialog('archive')}
      handleConfirm={handleArchive}
      disabled={updateApiKeyStatus.isPending}
      title={
        <span className='text-orange-600'>
          <IconArchive className='mr-1 inline-block stroke-orange-600' size={18} />
          {t('apikeys.dialogs.archive.title')}
        </span>
      }
      desc={getDescription()}
      confirmText={t('common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
