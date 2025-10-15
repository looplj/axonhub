import { useUsers } from '../context/users-context'
import { UsersActionDialog } from './users-action-dialog'
import { UsersAddToProjectDialog } from './users-add-to-project-dialog'
import { UsersDeleteDialog } from './users-delete-dialog'

export function UsersDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useUsers()
  return (
    <>
      <UsersAddToProjectDialog key='user-add-to-project' open={open === 'add'} onOpenChange={() => setOpen('add')} />

      {currentRow && (
        <>
          <UsersActionDialog
            key={`user-edit-${currentRow.id}`}
            open={open === 'edit'}
            onOpenChange={() => {
              setOpen('edit')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />

          <UsersDeleteDialog
            key={`user-remove-${currentRow.id}`}
            open={open === 'remove'}
            onOpenChange={() => {
              setOpen('remove')
              setTimeout(() => {
                setCurrentRow(null)
              }, 500)
            }}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
