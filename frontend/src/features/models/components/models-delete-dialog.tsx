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
import { useModelsContext } from '../context/models-context'
import { useDeleteModel } from '../data/models'

export function ModelsDeleteDialog() {
  const { isDialogOpen, closeDialog, selectedModel } = useModelsContext()
  const deleteModel = useDeleteModel()
  const [isDeleting, setIsDeleting] = useState(false)

  const handleDelete = async () => {
    if (!selectedModel) return

    setIsDeleting(true)
    try {
      await deleteModel.mutateAsync(selectedModel.id)
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
          <DialogTitle>删除模型</DialogTitle>
          <DialogDescription>
            确定要删除模型 "{selectedModel?.name}" 吗？此操作无法撤销。
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