'use client';

import { useEffect, useState } from 'react';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useDeleteChannel } from '../data/channels';
import { Channel } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

// A fresh random 4-digit code is generated every time the dialog opens, so a
// stale code from a previous attempt can never authorise a delete.
function generateConfirmationCode(): string {
  return Math.floor(1000 + Math.random() * 9000).toString();
}

export function ChannelsDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const [confirmationCode, setConfirmationCode] = useState('');

  useEffect(() => {
    if (open) {
      setConfirmationCode(generateConfirmationCode());
      setValue('');
    }
  }, [open]);

  const deleteChannel = useDeleteChannel();
  const isCodeMatched = value.trim() === confirmationCode && confirmationCode !== '';

  const handleDelete = async () => {
    if (!isCodeMatched) return;

    try {
      await deleteChannel.mutateAsync(currentRow.id);
      onOpenChange(false);
      setValue('');
    } catch {
      // The mutation surfaces its own error toast; keep the dialog open so the
      // user can retry.
    }
  };

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(state) => {
        if (!state) setValue('');
        onOpenChange(state);
      }}
      handleConfirm={handleDelete}
      disabled={!isCodeMatched || deleteChannel.isPending}
      title={
        <span className='text-destructive'>
          <IconAlertTriangle className='stroke-destructive mr-1 inline-block' size={18} /> {t('channels.dialogs.delete.title')}
        </span>
      }
      desc={
        <div className='space-y-4'>
          <Alert variant='destructive'>
            <IconAlertTriangle className='h-4 w-4' />
            <AlertTitle>{t('channels.dialogs.delete.warning')}</AlertTitle>
            <AlertDescription>{t('channels.dialogs.delete.warningTitle')}</AlertDescription>
          </Alert>
          <div className='space-y-2'>
            <Label htmlFor='channel-delete-confirmation'>
              {t('channels.dialogs.delete.confirmLabel')}
            </Label>
            <div
              className='bg-muted flex items-center justify-center rounded-md border py-3 font-mono text-2xl font-semibold tracking-[0.5em] tabular-nums select-all'
              aria-label={confirmationCode}
            >
              {confirmationCode}
            </div>
            <Input
              id='channel-delete-confirmation'
              autoComplete='off'
              inputMode='numeric'
              maxLength={4}
              placeholder={t('channels.dialogs.delete.confirmPlaceholder')}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              className='text-center font-mono tracking-[0.5em]'
            />
          </div>
        </div>
      }
      confirmText={deleteChannel.isPending ? t('channels.dialogs.delete.deletingButton') : t('channels.dialogs.delete.confirmButton')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
