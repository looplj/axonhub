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
import type { Agent } from '../data/schema';
import { useUpdateAgent } from '../data/agents';

interface AgentsStatusDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Agent;
}

export function AgentsStatusDialog({ open, onOpenChange, currentRow }: AgentsStatusDialogProps) {
  const { t } = useTranslation();
  const updateAgent = useUpdateAgent();

  const newStatus = currentRow.status === 'enabled' ? 'disabled' : 'enabled';

  const handleConfirm = useCallback(async () => {
    await updateAgent.mutateAsync({
      id: currentRow.id,
      input: { status: newStatus },
    });
    onOpenChange(false);
  }, [updateAgent, currentRow.id, newStatus, onOpenChange]);

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('agents.dialogs.statusChange.title')}</AlertDialogTitle>
          <AlertDialogDescription>{t(`agents.dialogs.statusChange.description.${newStatus}`, { name: currentRow.name })}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={updateAgent.isPending}>
            {t('common.buttons.confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

