import { ApiKeysCreateDialog } from './apikeys-create-dialog'
import { ApiKeysEditDialog } from './apikeys-edit-dialog'
// import { ApiKeysDeleteDialog } from './apikeys-delete-dialog'
import { ApiKeysStatusDialog } from './apikeys-status-dialog'
import { ApiKeysViewDialog } from './apikeys-view-dialog'

export function ApiKeysDialogs() {
  return (
    <>
      <ApiKeysCreateDialog />
      <ApiKeysEditDialog />
      {/* <ApiKeysDeleteDialog /> */}
      <ApiKeysStatusDialog />
      <ApiKeysViewDialog />
    </>
  )
}