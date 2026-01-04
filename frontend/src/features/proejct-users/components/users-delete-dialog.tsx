'use client';

import { useState } from 'react';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { User } from '../data/schema';
import { useRemoveUserFromProject } from '../data/users';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: User;
}

export function UsersDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const [confirmText, setConfirmText] = useState('');
  const removeUser = useRemoveUserFromProject();

  const fullName = `${currentRow.firstName} ${currentRow.lastName}`;

  const handleRemove = async () => {
    if (confirmText.trim() !== fullName) return;

    try {
      await removeUser.mutateAsync(currentRow.id);
      toast.success(t('users.messages.removeFromProjectSuccess'));
      onOpenChange(false);
      setConfirmText('');
    } catch (error) {
      toast.error(t('common.errors.somethingWentWrong'));
    }
  };

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(state) => {
        onOpenChange(state);
        if (!state) setConfirmText('');
      }}
      handleConfirm={handleRemove}
      disabled={confirmText.trim() !== fullName || removeUser.isPending}
      title={
        <span className='text-destructive'>
          <IconAlertTriangle className='stroke-destructive mr-1 inline-block' size={18} /> {t('users.dialogs.remove.title')}
        </span>
      }
      desc={
        <div className='space-y-4'>
          <p className='mb-2'>{t('users.dialogs.remove.description', { name: fullName })}</p>

          <Label className='my-2'>
            {t('users.dialogs.remove.confirmLabel')}
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={t('users.dialogs.remove.confirmPlaceholder')}
              data-testid='remove-confirmation-input'
            />
          </Label>

          <Alert variant='destructive'>
            <AlertTitle>{t('users.dialogs.remove.warningTitle')}</AlertTitle>
            <AlertDescription>{t('users.dialogs.remove.warningDescription')}</AlertDescription>
          </Alert>
        </div>
      }
      confirmText={removeUser.isPending ? t('users.buttons.removing') : t('users.buttons.remove')}
      destructive
      data-testid='remove-dialog'
    />
  );
}
