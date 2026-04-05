import { useAgentHosts } from '../context/agent-hosts-context';
import { AgentHostsActionDialog } from './agent-hosts-action-dialog';
import { AgentHostsBulkDeactivateDialog } from './agent-hosts-bulk-deactivate-dialog';

export function AgentHostsDialogs() {
  const { open, setOpen } = useAgentHosts();

  return (
    <>
      <AgentHostsActionDialog
        key="agent-host-add"
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
      />

      <AgentHostsBulkDeactivateDialog />
    </>
  );
}
