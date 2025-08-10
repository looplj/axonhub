'use client'

import { IconAlertTriangle } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { ApiKey } from '../data/schema'
import { useUpdateApiKeyStatus } from '../data/apikeys'
import { useApiKeysContext } from '../context/apikeys-context'

export function ApiKeysStatusDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const updateApiKeyStatus = useUpdateApiKeyStatus()

  if (!selectedApiKey) return null

  const handleStatusChange = async () => {
    const newStatus = selectedApiKey.status === 'enabled' ? 'disabled' : 'enabled'
    
    try {
      await updateApiKeyStatus.mutateAsync({
        id: selectedApiKey.id,
        status: newStatus,
      })
      closeDialog('status')
    } catch (error) {
      console.error('Failed to update API key status:', error)
    }
  }

  const isDisabling = selectedApiKey.status === 'enabled'

  return (
    <ConfirmDialog
      open={isDialogOpen.status}
      onOpenChange={() => closeDialog('status')}
      handleConfirm={handleStatusChange}
      disabled={updateApiKeyStatus.isPending}
      title={
        <span className={isDisabling ? 'text-destructive' : 'text-green-600'}>
          <IconAlertTriangle
            className={`${isDisabling ? 'stroke-destructive' : 'stroke-green-600'} mr-1 inline-block`}
            size={18}
          />
          {isDisabling ? t('apikeys.dialog.status.disableTitle') : t('apikeys.dialog.status.enableTitle')}
        </span>
      }
      desc={
        isDisabling
          ? t('apikeys.dialog.status.disableDescription', { name: selectedApiKey.name })
          : t('apikeys.dialog.status.enableDescription', { name: selectedApiKey.name })
      }
      confirmText={isDisabling ? t('apikeys.dialog.buttons.disable') : t('apikeys.dialog.buttons.enable')}
      cancelBtnText={t('apikeys.dialog.buttons.cancel')}
    />
  )
}