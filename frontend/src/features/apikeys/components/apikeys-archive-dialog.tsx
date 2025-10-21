'use client'

import { useTranslation } from 'react-i18next'
import { IconArchive, IconInfoCircle } from '@tabler/icons-react'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useUpdateApiKeyStatus } from '../data/apikeys'
import { useApiKeysContext } from '../context/apikeys-context'

export function ApiKeysArchiveDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const updateApiKeyStatus = useUpdateApiKeyStatus()

  if (!selectedApiKey) return null

  const handleArchive = async () => {
    try {
      await updateApiKeyStatus.mutateAsync({
        id: selectedApiKey.id,
        status: 'archived',
      })
      closeDialog('archive')
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  }

  const getDescription = () => {
    const baseDescription = t('apikeys.dialogs.status.archive.description', { name: selectedApiKey.name })
    const warningText = t('apikeys.dialogs.status.archive.warning')

    return (
      <div className="space-y-3">
        <p>{baseDescription}</p>
        <div className="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md">
          <div className="flex items-start space-x-2">
            <IconInfoCircle className="h-4 w-4 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
            <div className="text-sm text-blue-800 dark:text-blue-200">
              <p>{warningText}</p>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <ConfirmDialog
      open={isDialogOpen.archive}
      onOpenChange={() => closeDialog('archive')}
      handleConfirm={handleArchive}
      disabled={updateApiKeyStatus.isPending}
      title={
        <span className="text-orange-600">
          <IconArchive
            className="stroke-orange-600 mr-1 inline-block"
            size={18}
          />
          {t('apikeys.dialogs.status.archive.title')}
        </span>
      }
      desc={getDescription()}
      confirmText={t('common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  )
}