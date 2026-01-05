import { useUsers } from '../context/users-context';
import { ProjectUserActionDialog } from './project-user-action-dialog';
import { UsersDeleteDialog } from './users-delete-dialog';

export function UsersDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useUsers();
  return (
    <>
      <ProjectUserActionDialog key='user-add' open={open === 'add'} onOpenChange={() => setOpen('add')} />

      {currentRow && (
        <>
          <ProjectUserActionDialog
            key={`user-edit-${currentRow.id}`}
            open={open === 'edit'}
            onOpenChange={() => {
              setOpen('edit');
              setTimeout(() => {
                setCurrentRow(null);
              }, 500);
            }}
            currentRow={currentRow}
          />

          <UsersDeleteDialog
            key={`user-remove-${currentRow.id}`}
            open={open === 'remove'}
            onOpenChange={() => {
              setOpen('remove');
              setTimeout(() => {
                setCurrentRow(null);
              }, 500);
            }}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  );
}
