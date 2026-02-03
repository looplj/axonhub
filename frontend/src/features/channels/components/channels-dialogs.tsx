import { useChannels } from '../context/channels-context';
import { ChannelsActionDialog } from './channels-action-dialog';
import { ChannelsArchiveDialog } from './channels-archive-dialog';
import { ChannelsBulkApplyTemplateDialog } from './channels-bulk-apply-template-dialog';
import { ChannelsBulkArchiveDialog } from './channels-bulk-archive-dialog';
import { ChannelsBulkDeleteDialog } from './channels-bulk-delete-dialog';
import { ChannelsBulkDisableDialog } from './channels-bulk-disable-dialog';
import { ChannelsBulkEnableDialog } from './channels-bulk-enable-dialog';
import { ChannelsBulkImportDialog } from './channels-bulk-import-dialog';
import { ChannelsBulkOrderingDialog } from './channels-bulk-ordering-dialog';
import { ChannelsDeleteDialog } from './channels-delete-dialog';
import { ChannelsErrorResolvedDialog } from './channels-error-resolved-dialog';
import { ChannelsModelMappingDialog } from './channels-model-mapping-dialog';
import { ChannelsModelPriceDialog } from './channels-model-price-dialog';
import { ChannelsOverrideDialog } from './channels-override-dialog';
import { ChannelsProxyDialog } from './channels-proxy-dialog';
import { ChannelsStatusDialog } from './channels-status-dialog';
import { ChannelsTestDialog } from './channels-test-dialog';
import { ChannelsTransformOptionsDialog } from './channels-transform-options-dialog';
import { ChannelsSystemSettingsDialog } from './channels-system-settings-dialog';

export function ChannelsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow, selectedChannels } = useChannels();
  return (
    <>
      <ChannelsSystemSettingsDialog />

      <ChannelsActionDialog key='channel-add' open={open === 'add'} onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)} />

      <ChannelsBulkArchiveDialog />

      <ChannelsBulkDisableDialog />

      <ChannelsBulkEnableDialog />

      <ChannelsBulkDeleteDialog />

      <ChannelsBulkApplyTemplateDialog
        open={open === 'bulkApplyTemplate'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulkApplyTemplate' : null)}
        selectedChannels={selectedChannels}
      />

      <ChannelsBulkImportDialog isOpen={open === 'bulkImport'} onClose={() => setOpen(null)} />

      <ChannelsBulkOrderingDialog open={open === 'bulkOrdering'} onOpenChange={(isOpen) => setOpen(isOpen ? 'bulkOrdering' : null)} />

      {currentRow && (
        <>
          <ChannelsActionDialog
            key={`channel-edit-${currentRow.id}`}
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
            currentRow={currentRow}
          />

          <ChannelsActionDialog
            key={`channel-duplicate-${currentRow.id}`}
            open={open === 'duplicate'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('duplicate');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            duplicateFromRow={currentRow}
          />

          <ChannelsActionDialog
            key={`channel-view-models-${currentRow.id}`}
            open={open === 'viewModels'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('viewModels');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            currentRow={currentRow}
            showModelsPanel={true}
          />

          <ChannelsDeleteDialog
            key={`channel-delete-${currentRow.id}`}
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

          {/* <ChannelsSettingsDialog
            key={`channel-settings-${currentRow.id}`}
            open={open === 'settings'}
            onOpenChange={() => {
              setOpen('settings')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          /> */}

          <ChannelsModelMappingDialog
            key={`channel-model-mapping-${currentRow.id}`}
            open={open === 'modelMapping'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('modelMapping');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            currentRow={currentRow}
          />

          <ChannelsModelPriceDialog />

          <ChannelsOverrideDialog
            key={`channel-overrides-${currentRow.id}`}
            open={open === 'overrides'}
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

          <ChannelsProxyDialog
            key={`channel-proxy-${currentRow.id}`}
            open={open === 'proxy'}
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

          <ChannelsStatusDialog
            key={`channel-status-${currentRow.id}`}
            open={open === 'status'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('status');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            currentRow={currentRow}
          />

          <ChannelsArchiveDialog
            key={`channel-archive-${currentRow.id}`}
            open={open === 'archive'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('archive');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            currentRow={currentRow}
          />

          <ChannelsTestDialog
            key={`channel-test-${currentRow.id}`}
            open={open === 'test'}
            onOpenChange={(isOpen: boolean) => {
              if (isOpen) {
                setOpen('test');
              } else {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            channel={currentRow}
          />

          <ChannelsErrorResolvedDialog
            key={`channel-error-resolved-${currentRow.id}`}
            open={open === 'errorResolved'}
            onOpenChange={(isOpen) => {
              if (!isOpen) {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
          />

          <ChannelsTransformOptionsDialog
            key={`channel-transform-options-${currentRow.id}`}
            open={open === 'transformOptions'}
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
