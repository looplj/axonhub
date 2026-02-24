'use client';

import { useTranslation } from 'react-i18next';
import { IconBan } from '@tabler/icons-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useAgentRuntimes } from '../context/agent-runtimes-context';
import { useBulkUpdateAgentRuntimeStatus } from '../data/agent-runtimes';

export function AgentRuntimesBulkDeactivateDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedAgentRuntimes, resetRowSelection } = useAgentRuntimes();
  const bulkUpdateStatus = useBulkUpdateAgentRuntimeStatus();

  const handleConfirm = async () => {
    try {
      const ids = selectedAgentRuntimes.map((ar) => ar.id);
      await bulkUpdateStatus.mutateAsync({ ids, status: 'inactive' });
      setOpen(null);
      resetRowSelection();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={open === 'bulkDeactivate'}
      onOpenChange={() => setOpen(null)}
      handleConfirm={handleConfirm}
      disabled={bulkUpdateStatus.isPending}
      title={
        <span className="text-amber-600">
          <IconBan className="mr-1 inline-block" size={18} />{' '}
          {t('agentRuntimes.dialogs.bulkDeactivate.title')}
        </span>
      }
      desc={
        <div className="space-y-4">
          <Alert>
            <IconBan className="h-4 w-4" />
            <AlertTitle>{t('agentRuntimes.dialogs.bulkDeactivate.confirmation')}</AlertTitle>
            <AlertDescription>
              {t('agentRuntimes.dialogs.bulkDeactivate.description', {
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
        bulkUpdateStatus.isPending
          ? t('agentRuntimes.dialogs.bulkDeactivate.deactivatingButton')
          : t('agentRuntimes.dialogs.bulkDeactivate.confirmButton')
      }
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
