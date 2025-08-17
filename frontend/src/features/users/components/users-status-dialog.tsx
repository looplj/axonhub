'use client'

import { useState } from 'react'
import { IconUserCheck, IconUserOff } from '@tabler/icons-react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { User } from '../data/schema'
import { useUpdateUserStatus } from '../data/users'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: User
}

export function UsersStatusDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const updateUserStatus = useUpdateUserStatus()
  const isActivated = currentRow.status === 'activated'
  const newStatus = isActivated ? 'deactivated' : 'activated'
  const actionText = isActivated ? t('users.actions.deactivate') : t('users.actions.activate')
  
  const handleStatusChange = async () => {
    try {
      await updateUserStatus.mutateAsync({
        id: currentRow.id,
        status: newStatus
      })
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to update user status:', error)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleStatusChange}
      disabled={updateUserStatus.isPending}
      title={
        <span className={isActivated ? 'text-destructive' : 'text-green-600'}>
          {isActivated ? (
            <IconUserOff className='mr-1 inline-block' size={18} />
          ) : (
            <IconUserCheck className='mr-1 inline-block' size={18} />
          )}
          {t('users.dialogs.statusChange.title', { action: actionText })}
        </span>
      }
      desc={
        <div className='space-y-2'>
          <p>
            {t('users.dialogs.statusChange.confirmMessage', { 
              action: actionText, 
              name: `${currentRow.firstName} ${currentRow.lastName}` 
            })}
          </p>
          <p className='text-sm text-muted-foreground'>
            {isActivated 
              ? t('users.dialogs.statusChange.deactivateWarning') 
              : t('users.dialogs.statusChange.activateInfo')
            }
          </p>
        </div>
      }
      confirmText={actionText}
      cancelBtnText={t('users.buttons.cancel')}
    />
  )
}