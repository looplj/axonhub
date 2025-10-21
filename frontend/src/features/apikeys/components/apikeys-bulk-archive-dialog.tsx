'use client'

import { IconAlertTriangle, IconArchive } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useApiKeysContext } from '../context/apikeys-context'
import { useBulkArchiveApiKeys } from '../data/apikeys'
import { ApiKey } from '../data/schema'

export function ApiKeysBulkArchiveDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog, selectedApiKeys, resetRowSelection } = useApiKeysContext()
  const bulkArchiveApiKeys = useBulkArchiveApiKeys()

  if (!selectedApiKeys || selectedApiKeys.length === 0) return null

  const handleBulkArchive = async () => {
    try {
      const ids = selectedApiKeys.map(apiKey => apiKey.id)
      await bulkArchiveApiKeys.mutateAsync(ids)
      closeDialog('bulkArchive')
      resetRowSelection() // 清空选中的行
    } catch (error) {
      console.error('Failed to bulk archive API keys:', error)
    }
  }

  return (
    <ConfirmDialog
      open={isDialogOpen.bulkArchive}
      onOpenChange={() => closeDialog('bulkArchive')}
      handleConfirm={handleBulkArchive}
      disabled={bulkArchiveApiKeys.isPending}
      title={
        <span className="text-destructive">
          <IconAlertTriangle className="stroke-destructive mr-1 inline-block" size={18} />
          {t('apikeys.dialogs.bulkArchive.title')}
        </span>
      }
      desc={t('apikeys.dialogs.bulkArchive.description', { count: selectedApiKeys.length })}
      confirmText={t('common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  )
}
