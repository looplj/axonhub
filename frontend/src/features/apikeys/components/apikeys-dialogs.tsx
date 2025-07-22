import { ApiKeysCreateDialog } from './apikeys-create-dialog'
import { ApiKeysEditDialog } from './apikeys-edit-dialog'
import { ApiKeysDeleteDialog } from './apikeys-delete-dialog'

export function ApiKeysDialogs() {
  return (
    <>
      <ApiKeysCreateDialog />
      <ApiKeysEditDialog />
      <ApiKeysDeleteDialog />
    </>
  )
}