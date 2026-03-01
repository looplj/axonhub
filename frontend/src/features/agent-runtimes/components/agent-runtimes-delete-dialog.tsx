'use client';

import { useState } from 'react';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useDeleteAgentRuntime } from '../data/agent-runtimes';
import { AgentRuntime } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: AgentRuntime;
}

export function AgentRuntimesDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const deleteAgentRuntime = useDeleteAgentRuntime();

  const isLocal = currentRow.type === 'local';

  if (isLocal) {
    return null;
  }

  const handleDelete = async () => {
    if (value.trim() !== currentRow.name) return;

    try {
      await deleteAgentRuntime.mutateAsync(currentRow.id);
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
      disabled={value.trim() !== currentRow.name || deleteAgentRuntime.isPending}
      title={
        <span className="text-destructive">
          <IconAlertTriangle className="stroke-destructive mr-1 inline-block" size={18} />{' '}
          {t('agentRuntimes.dialogs.delete.title')}
        </span>
      }
      desc={
        <div className="space-y-4">
          <Alert variant="destructive">
            <IconAlertTriangle className="h-4 w-4" />
            <AlertTitle>{t('agentRuntimes.dialogs.delete.warning')}</AlertTitle>
            <AlertDescription>{t('agentRuntimes.dialogs.delete.warningDescription')}</AlertDescription>
          </Alert>
          <div className="space-y-2">
            <Label htmlFor="agent-runtime-name">
              {t('agentRuntimes.dialogs.delete.confirmLabel')}{' '}
              <strong>{currentRow.name}</strong>{' '}
              {t('agentRuntimes.dialogs.delete.confirmLabelSuffix')}
            </Label>
            <Input
              id="agent-runtime-name"
              placeholder={currentRow.name}
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
          </div>
        </div>
      }
      confirmText={
        deleteAgentRuntime.isPending
          ? t('agentRuntimes.dialogs.delete.deletingButton')
          : t('agentRuntimes.dialogs.delete.confirmButton')
      }
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
