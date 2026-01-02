import { useApiKeysContext } from '../context/apikeys-context'
import { useApiKey, useUpdateApiKeyProfiles } from '../data/apikeys'
import { type UpdateApiKeyProfilesInput } from '../data/schema'
import { ApiKeysArchiveDialog } from './apikeys-archive-dialog'
import { ApiKeysCreateDialog } from './apikeys-create-dialog'
import { ApiKeysEditDialog } from './apikeys-edit-dialog'
import { ApiKeyProfilesDialog } from './apikeys-profiles-dialog'
// import { ApiKeysDeleteDialog } from './apikeys-delete-dialog'
import { ApiKeysStatusDialog } from './apikeys-status-dialog'
import { ApiKeysViewDialog } from './apikeys-view-dialog'
import { ApiKeysBulkDisableDialog } from './apikeys-bulk-disable-dialog'
import { ApiKeysBulkArchiveDialog } from './apikeys-bulk-archive-dialog'
import { ApiKeysBulkEnableDialog } from './apikeys-bulk-enable-dialog'

export function ApiKeysDialogs() {
  return (
    <>
      <ApiKeysCreateDialog />
      <ApiKeysEditDialog />
      {/* <ApiKeysDeleteDialog /> */}
      <ApiKeysStatusDialog />
      <ApiKeysViewDialog />
      <ApiKeysArchiveDialog />
      <ApiKeysProfilesDialogWrapper />
      <ApiKeysBulkDisableDialog />
      <ApiKeysBulkArchiveDialog />
      <ApiKeysBulkEnableDialog />
    </>
  )
}

function ApiKeysProfilesDialogWrapper() {
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const updateProfilesMutation = useUpdateApiKeyProfiles()
  const { data: apiKeyDetail } = useApiKey(selectedApiKey?.id || '')

  const handleSubmit = (data: UpdateApiKeyProfilesInput) => {
    if (!selectedApiKey?.id) return

    updateProfilesMutation.mutate(
      { id: selectedApiKey.id, input: data },
      {
        onSuccess: () => {
          closeDialog('profiles')
        },
      }
    )
  }

  return (
    <ApiKeyProfilesDialog
      open={isDialogOpen.profiles}
      onOpenChange={(open) => !open && closeDialog('profiles')}
      onSubmit={handleSubmit}
      loading={updateProfilesMutation.isPending}
      initialData={
        apiKeyDetail?.profiles
          ? {
              activeProfile:
                apiKeyDetail.profiles.activeProfile || apiKeyDetail.profiles.profiles?.[0]?.name || 'Default',
              profiles: apiKeyDetail.profiles.profiles || [],
            }
          : undefined
      }
    />
  )
}
