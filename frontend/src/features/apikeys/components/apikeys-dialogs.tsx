import { ApiKeysCreateDialog } from './apikeys-create-dialog'
import { ApiKeysEditDialog } from './apikeys-edit-dialog'
// import { ApiKeysDeleteDialog } from './apikeys-delete-dialog'
import { ApiKeysStatusDialog } from './apikeys-status-dialog'
import { ApiKeysViewDialog } from './apikeys-view-dialog'
import { ApiKeyProfilesDialog } from './apikeys-profiles-dialog'
import { useApiKeysContext } from '../context/apikeys-context'
import { type UpdateApiKeyProfilesInput } from '../data/schema'
import { useApiKey, useUpdateApiKeyProfiles } from '../data/apikeys'

export function ApiKeysDialogs() {
  return (
    <>
      <ApiKeysCreateDialog />
      <ApiKeysEditDialog />
      {/* <ApiKeysDeleteDialog /> */}
      <ApiKeysStatusDialog />
      <ApiKeysViewDialog />
      <ApiKeysProfilesDialogWrapper />
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
        }
      }
    )
  }

  return (
    <ApiKeyProfilesDialog
      open={isDialogOpen.profiles}
      onOpenChange={(open) => !open && closeDialog('profiles')}
      onSubmit={handleSubmit}
      loading={updateProfilesMutation.isPending}
      initialData={apiKeyDetail?.profiles ? {
        activeProfile: apiKeyDetail.profiles.activeProfile || apiKeyDetail.profiles.profiles?.[0]?.name || 'Default',
        profiles: apiKeyDetail.profiles.profiles || [],
      } : undefined}
    />
  )
}