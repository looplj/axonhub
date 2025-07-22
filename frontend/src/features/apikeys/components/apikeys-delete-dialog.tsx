import { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useApiKeysContext } from '../context/apikeys-context'
import { useDeleteApiKey } from '../data/apikeys'

export function ApiKeysDeleteDialog() {
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const deleteApiKey = useDeleteApiKey()
  const [isDeleting, setIsDeleting] = useState(false)

  const handleDelete = async () => {
    if (!selectedApiKey) return

    setIsDeleting(true)
    try {
      await deleteApiKey.mutateAsync(selectedApiKey.id)
      closeDialog('delete')
    } catch (error) {
      // Error is handled by the mutation
    } finally {
      setIsDeleting(false)
    }
  }

  const handleClose = () => {
    closeDialog('delete')
  }

  return (
    <Dialog open={isDialogOpen.delete} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[425px]'>
        <DialogHeader>
          <DialogTitle>删除 API Key</DialogTitle>
          <DialogDescription>
            确定要删除 API Key "{selectedApiKey?.name}" 吗？此操作无法撤销。
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={handleClose}
            disabled={isDeleting}
          >
            取消
          </Button>
          <Button
            type='button'
            variant='destructive'
            onClick={handleDelete}
            disabled={isDeleting}
          >
            {isDeleting ? '删除中...' : '删除'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}