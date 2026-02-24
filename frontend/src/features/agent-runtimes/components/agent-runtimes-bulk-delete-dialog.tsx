'use client';

import { useTranslation } from 'react-i18next';
import { IconAlertTriangle } from '@tabler/icons-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useAgentRuntimes } from '../context/agent-runtimes-context';
import { useBulkDeleteAgentRuntimes } from '../data/agent-runtimes';

export function AgentRuntimesBulkDeleteDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedAgentRuntimes, resetRowSelection } = useAgentRuntimes();
  const bulkDelete = useBulkDeleteAgentRuntimes();

  const handleConfirm = async () => {
    try {
      const ids = selectedAgentRuntimes.map((ar) => ar.id);
      await bulkDelete.mutateAsync(ids);
      setOpen(null);
      resetRowSelection();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={open === 'bulkDelete'}
      onOpenChange={() => setOpen(null)}
      handleConfirm={handleConfirm}
      disabled={bulkDelete.isPending}
      title={
        <span className="text-destructive">
          <IconAlertTriangle className="stroke-destructive mr-1 inline-block" size={18} />{' '}
          {t('agentRuntimes.dialogs.bulkDelete.title')}
        </span>
      }
      desc={
        <div className="space-y-4">
          <Alert variant="destructive">
            <IconAlertTriangle className="h-4 w-4" />
            <AlertTitle>{t('agentRuntimes.dialogs.bulkDelete.warning')}</AlertTitle>
            <AlertDescription>
              {t('agentRuntimes.dialogs.bulkDelete.warningDescription', {
                count: selectedAgentRuntimes.length,
              })}
            </AlertDescription>
          </Alert>
          <div className="max-h-32 overflow-y-auto rounded-md border p-2">
            <ul className="space-y-1">
              {selectedAgentRuntimes.map((ar) => (
                <li key={ar.id} className="text-sm">
                  • {ar.name}
                </li>
              ))}
            </ul>
          </div>
        </div>
      }
      confirmText={
        bulkDelete.isPending
          ? t('agentRuntimes.dialogs.bulkDelete.deletingButton')
          : t('agentRuntimes.dialogs.bulkDelete.confirmButton')
      }
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
