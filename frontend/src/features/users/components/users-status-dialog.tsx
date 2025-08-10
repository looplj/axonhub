'use client'

import { useState } from 'react'
import { IconUserCheck, IconUserOff } from '@tabler/icons-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { User } from '../data/schema'
import { useUpdateUserStatus } from '../data/users'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: User
}

export function UsersStatusDialog({ open, onOpenChange, currentRow }: Props) {
  const updateUserStatus = useUpdateUserStatus()
  const isActivated = currentRow.status === 'activated'
  const newStatus = isActivated ? 'deactivated' : 'activated'
  const actionText = isActivated ? '停用' : '激活'
  
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
          {actionText}用户
        </span>
      }
      desc={
        <div className='space-y-2'>
          <p>
            您确定要{actionText}用户 <strong>{currentRow.firstName} {currentRow.lastName}</strong> 吗？
          </p>
          <p className='text-sm text-muted-foreground'>
            {isActivated 
              ? '停用后，该用户将无法登录系统。' 
              : '激活后，该用户将能够正常登录系统。'
            }
          </p>
        </div>
      }
      confirmText={actionText}
      cancelBtnText='取消'
    />
  )
}