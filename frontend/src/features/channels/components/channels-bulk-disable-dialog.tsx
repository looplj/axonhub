'use client'

import { IconAlertTriangle, IconBan } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useChannels } from '../context/channels-context'
import { useBulkDisableChannels } from '../data/channels'

export function ChannelsBulkDisableDialog() {
  const { t } = useTranslation()
  const { open, setOpen, selectedChannels, resetRowSelection, setSelectedChannels } = useChannels()
  const bulkDisableChannels = useBulkDisableChannels()

  const isDialogOpen = open === 'bulkDisable'
  const selectedCount = selectedChannels.length

  if (selectedCount === 0 && !isDialogOpen) {
    return null
  }

  const handleConfirm = async () => {
    try {
      const ids = selectedChannels.map((channel) => channel.id)
      if (ids.length === 0) {
        return
      }

      await bulkDisableChannels.mutateAsync(ids)
      resetRowSelection()
      setSelectedChannels([])
      setOpen(null)
    } catch (error) {
      console.error('Failed to bulk disable channels:', error)
    }
  }

  return (
    <ConfirmDialog
      open={isDialogOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          setOpen(null)
        } else {
          setOpen('bulkDisable')
        }
      }}
      handleConfirm={handleConfirm}
      disabled={selectedCount === 0}
      isLoading={bulkDisableChannels.isPending}
      confirmText={t('common.buttons.disable')}
      cancelBtnText={t('common.buttons.cancel')}
      title={
        <span className='text-destructive flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('channels.dialogs.bulkDisable.title')}
        </span>
      }
      desc={t('channels.dialogs.bulkDisable.description', { count: selectedCount })}
    >
      <div className='flex items-start gap-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm dark:border-amber-900 dark:bg-amber-900/20'>
        <IconBan className='mt-0.5 h-4 w-4 text-amber-600 dark:text-amber-400' />
        <div className='space-y-1 text-left'>
          <p>{t('channels.dialogs.bulkDisable.warning')}</p>
        </div>
      </div>
    </ConfirmDialog>
  )
}
