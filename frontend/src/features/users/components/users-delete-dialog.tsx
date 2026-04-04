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
import { useDeleteUser } from '../data/users';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: User;
}

export function UsersDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const deleteUser = useDeleteUser();

  const fullName = `${currentRow.firstName} ${currentRow.lastName}`;

  const handleDelete = async () => {
    if (value.trim() !== fullName) return;

    try {
      await deleteUser.mutateAsync(currentRow.id);
      toast.success(t('common.success.userDeleted'));
      onOpenChange(false);
      setValue('');
    } catch (error) {
      toast.error(t('common.errors.somethingWentWrong'));
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
      disabled={value.trim() !== fullName || deleteUser.isPending}
      title={
        <span className='text-destructive'>
          <IconAlertTriangle className='stroke-destructive mr-1 inline-block' size={18} /> {t('users.dialogs.delete.title')}
        </span>
      }
      desc={
        <div className='space-y-4'>
          <Alert variant='destructive'>
            <IconAlertTriangle className='h-4 w-4' />
            <AlertTitle>{t('users.dialogs.delete.warning')}</AlertTitle>
            <AlertDescription>{t('users.dialogs.delete.warningTitle')}</AlertDescription>
          </Alert>
          <div className='space-y-2'>
            <Label htmlFor='user-fullname'>
              {t('users.dialogs.delete.confirmLabel')} <strong>{fullName}</strong> {t('users.dialogs.delete.confirmLabelSuffix')}
            </Label>
            <Input
              id='user-fullname'
              placeholder={fullName}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              data-testid='delete-confirmation-input'
            />
          </div>
        </div>
      }
      confirmText={deleteUser.isPending ? t('users.dialogs.delete.deletingButton') : t('users.dialogs.delete.confirmButton')}
      cancelBtnText={t('common.buttons.cancel')}
      destructive
      data-testid='delete-dialog'
    />
  );
}
