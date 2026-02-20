import { AgentsActionDialog } from './agents-action-dialog';
import { AgentsDeleteDialog } from './agents-delete-dialog';
import { AgentsKeyDialog } from './agents-key-dialog';

export function AgentsDialogs() {
  return (
    <>
      <AgentsActionDialog />
      <AgentsDeleteDialog />
      <AgentsKeyDialog />
    </>
  );
}

