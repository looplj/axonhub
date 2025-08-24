'use client'

import { useTranslation } from 'react-i18next'
import { IconArchive, IconInfoCircle } from '@tabler/icons-react'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Channel } from '../data/schema'
import { useUpdateChannelStatus } from '../data/channels'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Channel
}

export function ChannelsArchiveDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateChannelStatus = useUpdateChannelStatus()

  const handleArchive = async () => {
    try {
      await updateChannelStatus.mutateAsync({
        id: currentRow.id,
        status: 'archived',
      })
      onOpenChange(false)
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  }

  const getDescription = () => {
    const baseDescription = t('channels.dialogs.status.archive.description', { name: currentRow.name })
    const warningText = t('channels.dialogs.status.archive.warning')
    
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
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleArchive}
      disabled={updateChannelStatus.isPending}
      title={
        <span className="text-orange-600">
          <IconArchive
            className="stroke-orange-600 mr-1 inline-block"
            size={18}
          />
          {t('channels.dialogs.status.archive.title')}
        </span>
      }
      desc={getDescription()}
      confirmText={t('channels.dialogs.status.archive.button')}
      cancelBtnText={t('channels.dialogs.buttons.cancel')}
    />
  )
}