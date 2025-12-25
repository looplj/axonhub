'use client'

import { IconAlertTriangle, IconBan } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useModels } from '../context/models-context'
import { useBulkDisableModels } from '../data/models'

export function ModelsBulkDisableDialog() {
  const { t } = useTranslation()
  const { open, setOpen, selectedModels, resetRowSelection, setSelectedModels } = useModels()
  const bulkDisableModels = useBulkDisableModels()

  const isDialogOpen = open === 'bulkDisable'
  const selectedCount = selectedModels.length

  if (selectedCount === 0 && !isDialogOpen) {
    return null
  }

  const handleConfirm = async () => {
    try {
      const ids = selectedModels.map((model) => model.id)
      if (ids.length === 0) {
        return
      }

      await bulkDisableModels.mutateAsync(ids)
      resetRowSelection()
      setSelectedModels([])
      setOpen(null)
    } catch (error) {
      console.error('Failed to bulk disable models:', error)
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
      isLoading={bulkDisableModels.isPending}
      confirmText={t('common.buttons.disable')}
      cancelBtnText={t('common.buttons.cancel')}
      title={
        <span className='text-destructive flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('models.dialogs.bulkDisable.title')}
        </span>
      }
      desc={t('models.dialogs.bulkDisable.description', { count: selectedCount })}
    >
      <div className='flex items-start gap-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm dark:border-amber-900 dark:bg-amber-900/20'>
        <IconBan className='mt-0.5 h-4 w-4 text-amber-600 dark:text-amber-400' />
        <div className='space-y-1 text-left'>
          <p>{t('models.dialogs.bulkDisable.warning')}</p>
        </div>
      </div>
    </ConfirmDialog>
  )
}
