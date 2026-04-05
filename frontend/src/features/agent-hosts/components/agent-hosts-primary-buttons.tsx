import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { PermissionGuard } from '@/components/permission-guard';
import { useAgentHosts } from '../context/agent-hosts-context';

export function AgentHostsPrimaryButtons() {
  const { t } = useTranslation();
  const { setOpen } = useAgentHosts();

  return (
    <div className="flex gap-2 overflow-x-auto md:overflow-x-visible">
      <PermissionGuard requiredScope="write_agents">
        <Button
          className="shrink-0 space-x-1"
          onClick={() => setOpen('add')}
          data-testid="add-agent-host-button"
        >
          <span>{t('agentHosts.addAgentHost')}</span>
          <IconPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  );
}
