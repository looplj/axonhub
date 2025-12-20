import { useTranslation } from 'react-i18next'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Model } from '../data/schema'
import { useUpdateModel } from '../data/models'

interface ModelsStatusDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: Model
}

export function ModelsStatusDialog({ open, onOpenChange, currentRow }: ModelsStatusDialogProps) {
  const { t } = useTranslation()
  const updateModel = useUpdateModel()

  const isEnabled = currentRow.status === 'enabled'
  const newStatus = isEnabled ? 'disabled' : 'enabled'

  const handleConfirm = async () => {
    try {
      await updateModel.mutateAsync({
        id: currentRow.id,
        input: { status: newStatus },
      })
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to update model status:', error)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {isEnabled ? t('models.dialogs.status.disableTitle') : t('models.dialogs.status.enableTitle')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isEnabled
              ? t('models.dialogs.status.disableDescription', { name: currentRow.name })
              : t('models.dialogs.status.enableDescription', { name: currentRow.name })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={updateModel.isPending}>
            {t('common.buttons.confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
