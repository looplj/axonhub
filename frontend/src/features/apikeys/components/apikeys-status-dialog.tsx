'use client'

import { IconAlertTriangle } from '@tabler/icons-react'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { ApiKey } from '../data/schema'
import { useUpdateApiKeyStatus } from '../data/apikeys'
import { useApiKeysContext } from '../context/apikeys-context'

export function ApiKeysStatusDialog() {
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
          {isDisabling ? '停用API密钥' : '激活API密钥'}
        </span>
      }
      desc={
        isDisabling
          ? `确定要停用API密钥 "${selectedApiKey.name}" 吗？停用后该密钥将无法使用。`
          : `确定要激活API密钥 "${selectedApiKey.name}" 吗？激活后该密钥将可以正常使用。`
      }
      confirmText={isDisabling ? '停用' : '激活'}
      cancelBtnText='取消'
    />
  )
}