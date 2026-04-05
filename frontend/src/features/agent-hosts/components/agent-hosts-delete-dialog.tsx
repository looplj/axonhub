'use client';

import { useState } from 'react';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useDeleteAgentHost } from '../data/agent-hosts';
import { AgentHost } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: AgentHost;
}

export function AgentHostsDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const deleteAgentHost = useDeleteAgentHost();

  const isLocal = currentRow.type === 'local';

  if (isLocal) {
    return null;
  }

  const handleDelete = async () => {
    if (value.trim() !== currentRow.name) return;

    try {
      await deleteAgentHost.mutateAsync(currentRow.id);
      onOpenChange(false);
      setValue('');
    } catch (error) {
      // Error is handled by the mutation
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
      disabled={value.trim() !== currentRow.name || deleteAgentHost.isPending}
      title={
        <span className="text-destructive">
          <IconAlertTriangle className="stroke-destructive mr-1 inline-block" size={18} />{' '}
          {t('agentHosts.dialogs.delete.title')}
        </span>
      }
      desc={
        <div className="space-y-4">
          <Alert variant="destructive">
            <IconAlertTriangle className="h-4 w-4" />
            <AlertTitle>{t('agentHosts.dialogs.delete.warning')}</AlertTitle>
            <AlertDescription>{t('agentHosts.dialogs.delete.warningDescription')}</AlertDescription>
          </Alert>
          <div className="space-y-2">
            <Label htmlFor="agent-host-name">
              {t('agentHosts.dialogs.delete.confirmLabel')}{' '}
              <strong>{currentRow.name}</strong>{' '}
              {t('agentHosts.dialogs.delete.confirmLabelSuffix')}
            </Label>
            <Input
              id="agent-host-name"
              placeholder={currentRow.name}
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
          </div>
        </div>
      }
      confirmText={
        deleteAgentHost.isPending
          ? t('agentHosts.dialogs.delete.deletingButton')
          : t('agentHosts.dialogs.delete.confirmButton')
      }
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
