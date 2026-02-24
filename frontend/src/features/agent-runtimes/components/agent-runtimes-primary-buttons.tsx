import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { PermissionGuard } from '@/components/permission-guard';
import { useAgentRuntimes } from '../context/agent-runtimes-context';

export function AgentRuntimesPrimaryButtons() {
  const { t } = useTranslation();
  const { setOpen } = useAgentRuntimes();

  return (
    <div className="flex gap-2 overflow-x-auto md:overflow-x-visible">
      <PermissionGuard requiredScope="write_agents">
        <Button
          className="shrink-0 space-x-1"
          onClick={() => setOpen('add')}
          data-testid="add-agent-runtime-button"
        >
          <span>{t('agentRuntimes.addAgentRuntime')}</span>
          <IconPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  );
}
