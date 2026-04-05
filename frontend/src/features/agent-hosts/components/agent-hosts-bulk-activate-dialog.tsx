'use client';

import { useTranslation } from 'react-i18next';
import { IconCheck } from '@tabler/icons-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useAgentHosts } from '../context/agent-hosts-context';
import { useBulkUpdateAgentHostStatus } from '../data/agent-hosts';

export function AgentHostsBulkActivateDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedAgentHosts, resetRowSelection } = useAgentHosts();
  const bulkUpdateStatus = useBulkUpdateAgentHostStatus();

  const handleConfirm = async () => {
    try {
      const ids = selectedAgentHosts.map((ar) => ar.id);
      await bulkUpdateStatus.mutateAsync({ ids, status: 'active' });
      setOpen(null);
      resetRowSelection();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={open === 'bulkActivate'}
      onOpenChange={() => setOpen(null)}
      handleConfirm={handleConfirm}
      disabled={bulkUpdateStatus.isPending}
      title={
        <span className="text-green-600">
          <IconCheck className="mr-1 inline-block" size={18} />{' '}
          {t('agentHosts.dialogs.bulkActivate.title')}
        </span>
      }
      desc={
        <div className="space-y-4">
          <Alert>
            <IconCheck className="h-4 w-4" />
            <AlertTitle>{t('agentHosts.dialogs.bulkActivate.confirmation')}</AlertTitle>
            <AlertDescription>
              {t('agentHosts.dialogs.bulkActivate.description', {
                count: selectedAgentHosts.length,
              })}
            </AlertDescription>
          </Alert>
          <div className="max-h-32 overflow-y-auto rounded-md border p-2">
            <ul className="space-y-1">
              {selectedAgentHosts.map((ar) => (
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
          ? t('agentHosts.dialogs.bulkActivate.activatingButton')
          : t('agentHosts.dialogs.bulkActivate.confirmButton')
      }
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
