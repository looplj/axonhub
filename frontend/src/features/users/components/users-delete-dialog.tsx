'use client'

import { useState } from 'react'
import { IconAlertTriangle } from '@tabler/icons-react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { User } from '../data/schema'
import { useDeleteUser } from '../data/users'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: User
}

export function UsersDeleteDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const [confirmText, setConfirmText] = useState('')
  const deleteUser = useDeleteUser()
  
  const fullName = `${currentRow.firstName} ${currentRow.lastName}`

  const handleDelete = async () => {
    if (confirmText.trim() !== fullName) return

    try {
      await deleteUser.mutateAsync(currentRow.id)
      toast.success(t('common.success.userDeleted'))
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to delete user:', error)
      toast.error(t('common.errors.userDeleteFailed'))
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleDelete}
      disabled={confirmText.trim() !== fullName || deleteUser.isPending}
      title={
        <span className='text-destructive'>
          <IconAlertTriangle
            className='stroke-destructive mr-1 inline-block'
            size={18}
          />{' '}
          Delete User
        </span>
      }
      desc={
        <div className='space-y-4'>
          <p className='mb-2'>
            Are you sure you want to delete{' '}
            <span className='font-bold'>{fullName}</span>?
            <br />
            This action will permanently remove the user from the system. This cannot be undone.
          </p>

          <Label className='my-2'>
            Full Name:
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder='Enter full name to confirm deletion.'
              data-testid="delete-confirmation-input"
            />
          </Label>

          <Alert variant='destructive'>
            <AlertTitle>Warning!</AlertTitle>
            <AlertDescription>
              Please be carefull, this operation can not be rolled back.
            </AlertDescription>
          </Alert>
        </div>
      }
      confirmText={deleteUser.isPending ? 'Deleting...' : 'Delete'}
      destructive
      data-testid="delete-dialog"
    />
  )
}
