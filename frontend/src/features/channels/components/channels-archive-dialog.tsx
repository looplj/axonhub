'use client';

import { IconArchive, IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useUpdateChannelStatus } from '../data/channels';
import { Channel } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

export function ChannelsArchiveDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannelStatus = useUpdateChannelStatus();

  const handleArchive = async () => {
    try {
      await updateChannelStatus.mutateAsync({
        id: currentRow.id,
        status: 'archived',
      });
      onOpenChange(false);
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  };

  const getDescription = () => {
    const baseDescription = t('channels.dialogs.status.archive.description', { name: currentRow.name });
    const warningText = t('channels.dialogs.status.archive.warning');

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
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleArchive}
      disabled={updateChannelStatus.isPending}
      title={
        <span className='text-orange-600'>
          <IconArchive className='mr-1 inline-block stroke-orange-600' size={18} />
          {t('channels.dialogs.status.archive.title')}
        </span>
      }
      desc={getDescription()}
      confirmText={t('common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
