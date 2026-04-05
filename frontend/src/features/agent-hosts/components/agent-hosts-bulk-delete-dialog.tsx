'use client';

import { useTranslation } from 'react-i18next';
import { IconAlertTriangle } from '@tabler/icons-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useAgentHosts } from '../context/agent-hosts-context';
import { useBulkDeleteAgentHosts } from '../data/agent-hosts';

export function AgentHostsBulkDeleteDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedAgentHosts, resetRowSelection } = useAgentHosts();
  const bulkDelete = useBulkDeleteAgentHosts();

  const deletableAgentHosts = selectedAgentHosts.filter((ar) => ar.type !== 'local');

  const handleConfirm = async () => {
    try {
      const ids = deletableAgentHosts.map((ar) => ar.id);
      if (ids.length === 0) {
        setOpen(null);
        return;
      }
      await bulkDelete.mutateAsync(ids);
      setOpen(null);
      resetRowSelection();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  if (deletableAgentHosts.length === 0) {
    return null;
  }

  return (
    <ConfirmDialog
      open={open === 'bulkDelete'}
      onOpenChange={() => setOpen(null)}
      handleConfirm={handleConfirm}
      disabled={bulkDelete.isPending}
      title={
        <span className="text-destructive">
          <IconAlertTriangle className="stroke-destructive mr-1 inline-block" size={18} />{' '}
          {t('agentHosts.dialogs.bulkDelete.title')}
        </span>
      }
      desc={
        <div className="space-y-4">
          <Alert variant="destructive">
            <IconAlertTriangle className="h-4 w-4" />
            <AlertTitle>{t('agentHosts.dialogs.bulkDelete.warning')}</AlertTitle>
            <AlertDescription>
              {t('agentHosts.dialogs.bulkDelete.warningDescription', {
                count: deletableAgentHosts.length,
              })}
            </AlertDescription>
          </Alert>
          <div className="max-h-32 overflow-y-auto rounded-md border p-2">
            <ul className="space-y-1">
              {deletableAgentHosts.map((ar) => (
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
          ? t('agentHosts.dialogs.bulkDelete.deletingButton')
          : t('agentHosts.dialogs.bulkDelete.confirmButton')
      }
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
