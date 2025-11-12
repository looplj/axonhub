import { useChannels } from '../context/channels-context'
import { ChannelsActionDialog } from './channels-action-dialog'
import { ChannelsSettingsDialog } from './channels-settings-dialog'
import { ChannelsModelMappingDialog } from './channels-model-mapping-dialog'
import { ChannelsOverrideParametersDialog } from './channels-override-parameters-dialog'
import { ChannelsStatusDialog } from './channels-status-dialog'
import { ChannelsArchiveDialog } from './channels-archive-dialog'
import { ChannelsDeleteDialog } from './channels-delete-dialog'
import { ChannelsTestDialog } from './channels-test-dialog'
import { ChannelsBulkImportDialog } from './channels-bulk-import-dialog'
import { ChannelsBulkOrderingDialog } from './channels-bulk-ordering-dialog'
import { ChannelsBulkArchiveDialog } from './channels-bulk-archive-dialog'
import { ChannelsBulkDisableDialog } from './channels-bulk-disable-dialog'
import { ChannelsBulkEnableDialog } from './channels-bulk-enable-dialog'
import { ChannelsBulkDeleteDialog } from './channels-bulk-delete-dialog'

export function ChannelsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useChannels()
  return (
    <>
      <ChannelsActionDialog
        key='channel-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
      />

      <ChannelsBulkArchiveDialog />

      <ChannelsBulkDisableDialog />

      <ChannelsBulkEnableDialog />

      <ChannelsBulkDeleteDialog />

      <ChannelsBulkImportDialog
        isOpen={open === 'bulkImport'}
        onClose={() => setOpen(null)}
      />

      <ChannelsBulkOrderingDialog
        open={open === 'bulkOrdering'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulkOrdering' : null)}
      />

      {currentRow && (
        <>
          <ChannelsActionDialog
            key={`channel-edit-${currentRow.id}`}
            open={open === 'edit'}
            onOpenChange={() => {
              setOpen('edit')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />

          <ChannelsDeleteDialog
            key={`channel-delete-${currentRow.id}`}
            open={open === 'delete'}
            onOpenChange={(isOpen) => {
              if (!isOpen) {
                setOpen(null)
                setTimeout(() => {
                  setCurrentRow(null)
                }, 500)
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
            onOpenChange={() => {
              setOpen('modelMapping')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />

          <ChannelsOverrideParametersDialog
            key={`channel-override-parameters-${currentRow.id}`}
            open={open === 'overrideParameters'}
            onOpenChange={() => {
              setOpen('overrideParameters')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />

          <ChannelsStatusDialog
            key={`channel-status-${currentRow.id}`}
            open={open === 'status'}
            onOpenChange={() => {
              setOpen('status')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />

          <ChannelsArchiveDialog
            key={`channel-archive-${currentRow.id}`}
            open={open === 'archive'}
            onOpenChange={() => {
              setOpen('archive')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />

          <ChannelsTestDialog
            key={`channel-test-${currentRow.id}`}
            open={open === 'test'}
            onOpenChange={() => {
              setOpen('test')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            channel={currentRow}
          />
        </>
      )}
    </>
  )
}