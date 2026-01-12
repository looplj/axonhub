import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { usePrompts } from '../context/prompts-context';
import { useBulkDisablePrompts } from '../data/prompts';

export function PromptsBulkDisableDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedPrompts, resetRowSelection } = usePrompts();
  const bulkDisableMutation = useBulkDisablePrompts();

  const handleConfirm = useCallback(async () => {
    const ids = selectedPrompts.map((prompt) => prompt.id);
    await bulkDisableMutation.mutateAsync(ids);
    setOpen(null);
    resetRowSelection?.();
  }, [selectedPrompts, bulkDisableMutation, setOpen, resetRowSelection]);

  return (
    <AlertDialog open={open === 'bulkDisable'} onOpenChange={(isOpen) => !isOpen && setOpen(null)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('prompts.dialogs.bulkDisable.title')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('prompts.dialogs.bulkDisable.description', { count: selectedPrompts.length })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={bulkDisableMutation.isPending}>
            {t('common.buttons.confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
