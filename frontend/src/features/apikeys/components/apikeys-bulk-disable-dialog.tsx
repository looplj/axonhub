'use client'

import { IconAlertTriangle } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useApiKeysContext } from '../context/apikeys-context'
import { useBulkDisableApiKeys } from '../data/apikeys'
import { ApiKey } from '../data/schema'

export function ApiKeysBulkDisableDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog, selectedApiKeys, resetRowSelection } = useApiKeysContext()
  const bulkDisableApiKeys = useBulkDisableApiKeys()

  if (!selectedApiKeys || selectedApiKeys.length === 0) return null

  const handleBulkDisable = async () => {
    try {
      const ids = selectedApiKeys.map(apiKey => apiKey.id)
      await bulkDisableApiKeys.mutateAsync(ids)
      closeDialog('bulkDisable')
      resetRowSelection() // 清空选中的行
    } catch (error) {
      console.error('Failed to bulk disable API keys:', error)
    }
  }

  return (
    <ConfirmDialog
      open={isDialogOpen.bulkDisable}
      onOpenChange={() => closeDialog('bulkDisable')}
      handleConfirm={handleBulkDisable}
      disabled={bulkDisableApiKeys.isPending}
      title={
        <span className="text-destructive">
          <IconAlertTriangle className="stroke-destructive mr-1 inline-block" size={18} />
          {t('apikeys.dialogs.bulkDisable.title')}
        </span>
      }
      desc={t('apikeys.dialogs.bulkDisable.description', { count: selectedApiKeys.length })}
      confirmText={t('common.buttons.disable')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  )
}
