import { useEffect } from 'react';
import { useChannels } from '../context/channels-context';
import { ChannelsActionDialog } from './channels-action-dialog';
import { ChannelsArchiveDialog } from './channels-archive-dialog';
import { ChannelsAvailabilityDialog } from './channels-availability-dialog';
import { ChannelsBulkApplyTemplateDialog } from './channels-bulk-apply-template-dialog';
import { ChannelsBulkClearTemplateDialog } from './channels-bulk-clear-template-dialog';
import { ChannelsBulkArchiveDialog } from './channels-bulk-archive-dialog';
import { ChannelsBulkDeleteDialog } from './channels-bulk-delete-dialog';
import { ChannelsBulkDisableDialog } from './channels-bulk-disable-dialog';
import { ChannelsBulkEnableDialog } from './channels-bulk-enable-dialog';
import { ChannelsBulkImportDialog } from './channels-bulk-import-dialog';
import { ChannelsBulkOrderingDialog } from './channels-bulk-ordering-dialog';
import { ChannelsBulkTestDialog } from './channels-bulk-test-dialog';
import { ChannelsDeleteDialog } from './channels-delete-dialog';
import { ChannelsDisabledAPIKeysDialog } from './channels-disabled-api-keys-dialog';
import { ChannelsErrorResolvedDialog } from './channels-error-resolved-dialog';
import { ChannelsModelMappingDialog } from './channels-model-mapping-dialog';
import { ChannelsModelPriceDialog } from './channels-model-price-dialog';
import { ChannelsOverrideDialog } from './channels-override-dialog';
import { ChannelsProxyDialog } from './channels-proxy-dialog';
import { ChannelsStatusDialog } from './channels-status-dialog';
import { ChannelsTestDialog } from './channels-test-dialog';
import { ChannelsTestHistoryDrawer } from './channels-test-history-drawer';
import { ChannelsAPIKeyManagementDialog } from './channels-api-key-management-dialog';
import { ChannelsRateLimitDialog } from './channels-rate-limit-dialog';
import { ChannelsTransformOptionsDialog } from './channels-transform-options-dialog';
import { ChannelsEndpointsDialog } from './channels-endpoints-dialog';
import { ChannelsSystemSettingsDialog } from './channels-system-settings-dialog';
import { useChannelDetails } from '../data/channels';

export function ChannelsDialogs() {
  const { open, setOpen, currentRow: partialCurrentRow, setCurrentRow, selectedChannels } = useChannels();
  const detailsQuery = useChannelDetails(partialCurrentRow?.id, {
    enabled: Boolean(partialCurrentRow && open),
  });

  useEffect(() => {
    if (detailsQuery.data && partialCurrentRow?.id === detailsQuery.data.id && partialCurrentRow !== detailsQuery.data) {
      setCurrentRow(detailsQuery.data);
    }
  }, [detailsQuery.data, partialCurrentRow, setCurrentRow]);

  // List rows intentionally contain only fields required by visible columns.
  // Delay row-scoped dialogs until the full snapshot has been loaded so hiding
  // a column never removes data from edit/configuration dialogs.
  const currentRow =
    partialCurrentRow &&
    (!open || detailsQuery.isError || detailsQuery.data === partialCurrentRow)
      ? (detailsQuery.data ?? partialCurrentRow)
      : null;
  return (
    <>
      <ChannelsSystemSettingsDialog />

      <ChannelsActionDialog key='channel-add' open={open === 'add'} onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)} />

      <ChannelsBulkArchiveDialog />

      <ChannelsBulkDisableDialog />

      <ChannelsBulkEnableDialog />

      <ChannelsBulkTestDialog />

      <ChannelsBulkDeleteDialog />

      <ChannelsBulkApplyTemplateDialog
        open={open === 'bulkApplyTemplate'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulkApplyTemplate' : null)}
        selectedChannels={selectedChannels}
      />

      <ChannelsBulkClearTemplateDialog />

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

          <ChannelsTestHistoryDrawer
            key={`channel-test-history-${currentRow.id}`}
            open={open === 'testHistory'}
            onOpenChange={(isOpen) => {
              if (isOpen) {
                setOpen('testHistory');
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

          <ChannelsRateLimitDialog
            key={`channel-rate-limit-${currentRow.id}`}
            open={open === 'rateLimit'}
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

          <ChannelsEndpointsDialog
            key={`channel-endpoints-${currentRow.id}`}
            open={open === 'endpoints'}
            onOpenChange={(isOpen) => {
              if (!isOpen) {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
            channel={currentRow}
          />

          <ChannelsDisabledAPIKeysDialog
            key={`channel-disabled-api-keys-${currentRow.id}`}
            open={open === 'disabledAPIKeys'}
            onOpenChange={(isOpen) => {
              if (!isOpen) {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
          />

          <ChannelsAvailabilityDialog
            key={`channel-availability-${currentRow.id}`}
            open={open === 'availability'}
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

          <ChannelsAPIKeyManagementDialog
            key={`channel-key-management-${currentRow.id}`}
            open={open === 'keyManagement'}
            onOpenChange={(isOpen) => {
              if (!isOpen) {
                setOpen(null);
                setTimeout(() => {
                  setCurrentRow(null);
                }, 500);
              }
            }}
          />
        </>
      )}
    </>
  );
}
