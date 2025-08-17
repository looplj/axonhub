import { Copy, Eye, EyeOff, AlertTriangle } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useApiKeysContext } from '../context/apikeys-context'

export function ApiKeysViewDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const [isVisible, setIsVisible] = useState(false)

  const copyToClipboard = () => {
    if (selectedApiKey?.key) {
      navigator.clipboard.writeText(selectedApiKey.key)
      toast.success(t('apikeys.messages.copied'))
    }
  }

  const maskedKey = selectedApiKey?.key
    ? selectedApiKey.key.replace(/./g, '*').slice(0, -4) + selectedApiKey.key.slice(-4)
    : ''

  return (
    <Dialog open={isDialogOpen.view} onOpenChange={() => closeDialog()}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t('apikeys.dialogs.view.title')}</DialogTitle>
          <DialogDescription>
            {t('apikeys.dialogs.view.description')}
          </DialogDescription>
        </DialogHeader>
        
        <Alert className="border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950">
          <AlertTriangle className="h-4 w-4 text-orange-600 dark:text-orange-400" />
          <AlertDescription className="text-orange-800 dark:text-orange-200">
            {t('apikeys.dialogs.view.warning')}
          </AlertDescription>
        </Alert>

        <div className="space-y-4">
          <div>
            <label className="text-sm font-medium">{t('apikeys.columns.name')}</label>
            <div className="mt-1 p-3 bg-muted rounded-md">
              {selectedApiKey?.name}
            </div>
          </div>
          
          <div>
            <label className="text-sm font-medium">{t('apikeys.columns.key')}</label>
            <div className="mt-1 flex items-center space-x-2">
              <code className="flex-1 p-3 bg-muted rounded-md font-mono text-sm break-all">
                {isVisible ? selectedApiKey?.key : maskedKey}
              </code>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setIsVisible(!isVisible)}
                className="flex-shrink-0"
              >
                {isVisible ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={copyToClipboard}
                className="flex-shrink-0"
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}