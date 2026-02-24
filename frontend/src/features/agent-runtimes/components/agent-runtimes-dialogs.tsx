import { useAgentRuntimes } from '../context/agent-runtimes-context';
import { AgentRuntimesActionDialog } from './agent-runtimes-action-dialog';
import { AgentRuntimesDeleteDialog } from './agent-runtimes-delete-dialog';
import { AgentRuntimesBulkDeleteDialog } from './agent-runtimes-bulk-delete-dialog';
import { AgentRuntimesBulkActivateDialog } from './agent-runtimes-bulk-activate-dialog';
import { AgentRuntimesBulkDeactivateDialog } from './agent-runtimes-bulk-deactivate-dialog';

export function AgentRuntimesDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useAgentRuntimes();

  return (
    <>
      <AgentRuntimesActionDialog
        key="agent-runtime-add"
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
      />

      <AgentRuntimesBulkDeleteDialog />
      <AgentRuntimesBulkActivateDialog />
      <AgentRuntimesBulkDeactivateDialog />

      {currentRow && (
        <>
          <AgentRuntimesActionDialog
            key={`agent-runtime-edit-${currentRow.id}`}
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

          <AgentRuntimesDeleteDialog
            key={`agent-runtime-delete-${currentRow.id}`}
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
