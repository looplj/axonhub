'use client'

import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
  const title = isDisabling ? t('channels.status.disable.title') : t('channels.status.enable.title')
  const description = isDisabling 
    ? t('channels.status.disable.description', { name: currentRow.name })
    : t('channels.status.enable.description', { name: currentRow.name })
  const actionText = isDisabling ? t('channels.status.disable.button') : t('channels.status.enable.button')

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
          {title}
        </span>
      }
      desc={description}
      confirmText={actionText}
      cancelBtnText={t('channels.dialog.buttons.cancel')}
    />
  )
}