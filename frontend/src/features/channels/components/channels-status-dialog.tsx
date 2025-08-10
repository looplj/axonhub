'use client'

import { IconAlertTriangle } from '@tabler/icons-react'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Channel } from '../data/schema'
import { useUpdateChannelStatus } from '../data/channels'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

export function ChannelsStatusDialog({ open, onOpenChange, currentRow }: Props) {
  const updateChannelStatus = useUpdateChannelStatus()

  const handleStatusChange = async () => {
    const newStatus = currentRow.status === 'enabled' ? 'disabled' : 'enabled'
    
    try {
      await updateChannelStatus.mutateAsync({
        id: currentRow.id,
        status: newStatus,
      })
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to update channel status:', error)
    }
  }

  const isDisabling = currentRow.status === 'enabled'

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleStatusChange}
      disabled={updateChannelStatus.isPending}
      title={
        <span className={isDisabling ? 'text-destructive' : 'text-green-600'}>
          <IconAlertTriangle
            className={`${isDisabling ? 'stroke-destructive' : 'stroke-green-600'} mr-1 inline-block`}
            size={18}
          />
          {isDisabling ? '禁用频道' : '启用频道'}
        </span>
      }
      desc={
        isDisabling
          ? `确定要禁用频道 "${currentRow.name}" 吗？禁用后该频道将无法使用。`
          : `确定要启用频道 "${currentRow.name}" 吗？启用后该频道将可以正常使用。`
      }
      confirmText={isDisabling ? '禁用' : '启用'}
      cancelBtnText='取消'
    />
  )
}