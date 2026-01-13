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
import { useDeletePrompt } from '../data/prompts';

export function PromptsDeleteDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow, resetRowSelection } = usePrompts();
  const deletePromptMutation = useDeletePrompt();

  const handleConfirm = useCallback(async () => {
    if (!currentRow) return;

    await deletePromptMutation.mutateAsync(currentRow.id);
    setOpen(null);
    resetRowSelection?.();
  }, [currentRow, deletePromptMutation, setOpen, resetRowSelection]);

  return (
    <AlertDialog open={open === 'delete'} onOpenChange={(isOpen) => !isOpen && setOpen(null)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('prompts.dialogs.delete.title')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('prompts.dialogs.delete.description', { name: currentRow?.name })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={deletePromptMutation.isPending} className='bg-destructive text-destructive-foreground hover:bg-destructive/90'>
            {t('common.buttons.delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
