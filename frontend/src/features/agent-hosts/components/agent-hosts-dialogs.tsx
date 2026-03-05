import { useAgentHosts } from '../context/agent-hosts-context';
import { AgentHostsActionDialog } from './agent-hosts-action-dialog';
import { AgentHostsDeleteDialog } from './agent-hosts-delete-dialog';
import { AgentHostsBulkDeleteDialog } from './agent-hosts-bulk-delete-dialog';
import { AgentHostsBulkActivateDialog } from './agent-hosts-bulk-activate-dialog';
import { AgentHostsBulkDeactivateDialog } from './agent-hosts-bulk-deactivate-dialog';

export function AgentHostsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useAgentHosts();

  return (
    <>
      <AgentHostsActionDialog
        key="agent-host-add"
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
      />

      <AgentHostsBulkDeleteDialog />
      <AgentHostsBulkActivateDialog />
      <AgentHostsBulkDeactivateDialog />

      {currentRow && (
        <>
          <AgentHostsActionDialog
            key={`agent-host-edit-${currentRow.id}`}
            currentRow={currentRow}
            open={open === 'edit'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('edit');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
          />

          <AgentHostsDeleteDialog
            key={`agent-host-delete-${currentRow.id}`}
            open={open === 'delete'}
            onOpenChange={(isOpen) => {
              if (!isOpen) {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  );
}
