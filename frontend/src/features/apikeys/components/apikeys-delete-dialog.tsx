import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
          <DialogTitle>{t('apikeys.dialog.delete.title')}</DialogTitle>
          <DialogDescription>
            {t('apikeys.dialog.delete.description', { name: selectedApiKey?.name })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={handleClose}
            disabled={isDeleting}
          >
            {t('apikeys.dialog.buttons.cancel')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            onClick={handleDelete}
            disabled={isDeleting}
          >
            {isDeleting ? t('apikeys.dialog.buttons.deleting') : t('apikeys.dialog.buttons.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}