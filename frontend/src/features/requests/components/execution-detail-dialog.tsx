import { format } from 'date-fns'
import { zhCN, enUS } from 'date-fns/locale'
import { Copy, Clock, Key, Database } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { RequestExecution } from '../data/schema'
import { useTranslation } from 'react-i18next'
import { useRequestPermissions } from '../../../gql/useRequestPermissions'
import { getStatusColor } from './help'

interface ExecutionDetailDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  execution: RequestExecution | null
}

export function ExecutionDetailDialog({
  open,
  onOpenChange,
  execution,
}: ExecutionDetailDialogProps) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const permissions = useRequestPermissions()

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  const formatJson = (data: any) => {
    try {
      if (typeof data === 'string') {
        return JSON.stringify(JSON.parse(data), null, 2)
      }
      return JSON.stringify(data, null, 2)
    } catch {
      return typeof data === 'string' ? data : JSON.stringify(data)
    }
  }


  if (!execution) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='w-[90vw] h-[90vh] max-w-none -mx-12 p-4 min-w-2xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Database className='h-5 w-5' />
            {t('requests.dialogs.executionDetail.title')}
          </DialogTitle>
        </DialogHeader>

        <div className='space-y-6'>
          {/* Basic Information */}
          <div className='space-y-4'>
            <div className='grid grid-cols-2 gap-4'>
              <div className='space-y-2'>
                <div className='flex items-center gap-2'>
                  <span className='text-sm font-medium'>{t('requests.dialogs.executionDetail.channel')}:</span>
                   <p className='text-muted-foreground text-sm'>
                  {permissions.canViewChannels 
                    ? (execution.channel?.name || t('requests.dialogs.executionDetail.unknown'))
                    : '-'
                  }
                </p>
                </div>
               
              </div>
              <div className='space-y-2'>
                <div className='flex items-center gap-2'>
                  <Key className='text-muted-foreground h-4 w-4' />
                  <span className='text-sm font-medium'>{t('requests.dialogs.executionDetail.modelId')}:</span>
                    <p className='text-muted-foreground text-sm'>
                  {execution.modelID || t('requests.dialogs.executionDetail.unknown')}
                </p>
                </div>
              
              </div>
              <div className='space-y-2'>
                <span className='text-sm font-medium'>{t('requests.dialogs.executionDetail.status')}:</span>
                <Badge className={getStatusColor(execution.status)}>
                  {t(`requests.status.${execution.status}`)}
                </Badge>
              </div>
              <div className='space-y-2'>
                <div className='flex items-center gap-2'>
                  <Clock className='text-muted-foreground h-4 w-4' />
                  <span className='text-sm font-medium'>{t('requests.dialogs.executionDetail.createdAt')}:</span>
                  <p className='text-muted-foreground text-sm'>
                    {format(
                      new Date(execution.createdAt),
                      'yyyy-MM-dd HH:mm:ss',
                      { locale }
                    )}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <Separator />

          {/* Request/Response Body */}
          <div className='grid grid-cols-1 gap-6 lg:grid-cols-2'>
            {/* Request Body */}
            <div className='space-y-3'>
              <div className='flex items-center justify-between'>
                <h4 className='text-sm font-medium'>{t('requests.dialogs.executionDetail.requestBody')}</h4>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    copyToClipboard(formatJson(execution.requestBody))
                  }
                >
                  <Copy className='mr-2 h-4 w-4' />
                  {t('requests.dialogs.jsonViewer.copy')}
                </Button>
              </div>
              <ScrollArea className='h-128 w-full rounded-xs border p-4'>
                <pre className='text-xs whitespace-pre-wrap font-mono min-w-full'>
                  {formatJson(execution.requestBody)}
                </pre>
              </ScrollArea>
            </div>

            {/* Response Body */}
            <div className='space-y-3'>
              <div className='flex items-center justify-between'>
                <h4 className='text-sm font-medium'>{t('requests.dialogs.executionDetail.responseBody')}</h4>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    copyToClipboard(formatJson(execution.responseBody || ''))
                  }
                  disabled={!execution.responseBody}
                >
                  <Copy className='mr-2 h-4 w-4' />
                  {t('requests.dialogs.jsonViewer.copy')}
                </Button>
              </div>
              <ScrollArea className='h-128 w-full rounded-xs border p-4'>
                <pre className='text-xs whitespace-pre-wrap font-mono min-w-full'>
                  {execution.responseBody
                    ? formatJson(execution.responseBody)
                    : t('requests.dialogs.executionDetail.noResponse')}
                </pre>
              </ScrollArea>
            </div>
          </div>

          {/* Error Message */}
          {execution.errorMessage && (
            <>
              <Separator />
              <div className='space-y-3'>
                <h4 className='text-sm font-medium text-red-600'>{t('requests.dialogs.executionDetail.errorMessage')}</h4>
                <ScrollArea className='h-64 w-full rounded-xs border bg-red-50 p-4'>
                  <pre className='text-xs whitespace-pre-wrap text-red-800'>
                    {execution.errorMessage}
                  </pre>
                </ScrollArea>
              </div>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}