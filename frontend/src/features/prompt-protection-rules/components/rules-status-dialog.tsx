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
import { PromptProtectionRule } from '../data/schema';
import { useUpdatePromptProtectionRuleStatus } from '../data/rules';

interface RulesStatusDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: PromptProtectionRule;
}

export function RulesStatusDialog({ open, onOpenChange, currentRow }: RulesStatusDialogProps) {
  const { t } = useTranslation();
  const updateStatusMutation = useUpdatePromptProtectionRuleStatus();
  const newStatus = currentRow.status === 'enabled' ? 'disabled' : 'enabled';

  const handleConfirm = useCallback(async () => {
    await updateStatusMutation.mutateAsync({
      id: currentRow.id,
      status: newStatus,
    });
    onOpenChange(false);
  }, [currentRow.id, newStatus, onOpenChange, updateStatusMutation]);

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('promptProtectionRules.dialogs.statusChange.title')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(`promptProtectionRules.dialogs.statusChange.description.${newStatus}`, { name: currentRow.name })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={updateStatusMutation.isPending}>
            {t('common.buttons.confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
