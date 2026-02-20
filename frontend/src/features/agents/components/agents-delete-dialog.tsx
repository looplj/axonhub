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
import { useAgents } from '../context/agents-context';
import { useDeleteAgent } from '../data/agents';

export function AgentsDeleteDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow, setCurrentRow } = useAgents();
  const deleteAgent = useDeleteAgent();

  const isOpen = open === 'delete' && !!currentRow;

  const handleConfirm = useCallback(async () => {
    if (!currentRow) return;
    await deleteAgent.mutateAsync(currentRow.id);
    setOpen(null);
    setCurrentRow(null);
  }, [currentRow, deleteAgent, setOpen, setCurrentRow]);

  if (!currentRow) return null;

  return (
    <AlertDialog
      open={isOpen}
      onOpenChange={(next) => {
        if (!next) {
          setOpen(null);
          setCurrentRow(null);
        }
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('agents.dialogs.delete.title')}</AlertDialogTitle>
          <AlertDialogDescription>{t('agents.dialogs.delete.description', { name: currentRow.name })}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={deleteAgent.isPending}>
            {t('common.buttons.confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

