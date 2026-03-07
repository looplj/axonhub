import { useCallback, useEffect } from 'react';
import { useMessageChannels } from '../context/message-channels-context';
import { useMessageChannel } from '../data/message-channels';
import { MessageChannelsFormDialog } from './message-channels-form-dialog';
import { MessageChannelsDeleteDialog } from './message-channels-delete-dialog';
import { MessageChannelsBulkDeleteDialog } from './message-channels-bulk-delete-dialog';
import { ManageBindingsDialog } from './manage-bindings-dialog';

export function MessageChannelsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useMessageChannels();
  const { data: refreshedChannel, refetch } = useMessageChannel(currentRow?.id || '');

  useEffect(() => {
    if (refreshedChannel) {
      setCurrentRow(refreshedChannel);
    }
  }, [refreshedChannel, setCurrentRow]);

  const handleRefreshBindings = useCallback(() => {
    refetch();
  }, [refetch]);

  return (
    <>
      <MessageChannelsFormDialog
        open={open === 'create' || open === 'edit'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'edit' ? currentRow : null}
      />
      {currentRow && open === 'delete' && (
        <MessageChannelsDeleteDialog
          open={open === 'delete'}
          onOpenChange={(isOpen) => !isOpen && setOpen(null)}
          currentRow={currentRow}
        />
      )}
      <MessageChannelsBulkDeleteDialog />
      {currentRow && (
        <ManageBindingsDialog
          open={open === 'manageBindings'}
          onOpenChange={(isOpen) => !isOpen && setOpen(null)}
          currentRow={currentRow}
          onRefresh={handleRefreshBindings}
        />
      )}
    </>
  );
}
