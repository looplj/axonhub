import { ModelsCreateDialog } from './models-create-dialog'
import { ModelsEditDialog } from './models-edit-dialog'
import { ModelsDeleteDialog } from './models-delete-dialog'

export function ModelsDialogs() {
  return (
    <>
      <ModelsCreateDialog />
      <ModelsEditDialog />
      <ModelsDeleteDialog />
    </>
  )
}