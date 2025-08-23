import { useChannels } from '../context/channels-context'
import { ChannelsActionDialog } from './channels-action-dialog'
import { ChannelsSettingsDialog } from './channels-settings-dialog'
import { ChannelsStatusDialog } from './channels-status-dialog'
import { ChannelsTestDialog } from './channels-test-dialog'

export function ChannelsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useChannels()
  return (
    <>
      <ChannelsActionDialog
        key='channel-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
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

          {/* <ChannelsDeleteDialog
            key={`channel-delete-${currentRow.id}`}
            open={open === 'delete'}
            onOpenChange={() => {
              setOpen('delete')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          /> */}

          <ChannelsSettingsDialog
            key={`channel-settings-${currentRow.id}`}
            open={open === 'settings'}
            onOpenChange={() => {
              setOpen('settings')
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