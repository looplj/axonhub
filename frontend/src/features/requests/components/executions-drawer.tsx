import { format } from 'date-fns'
import { zhCN, enUS } from 'date-fns/locale'
import { Eye, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { RequestExecution } from '../data/schema'
import { useChannels } from '@/features/channels/data'
import { getStatusColor } from './help'

interface ExecutionsDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  executions: RequestExecution[]
  onExecutionSelect: (execution: RequestExecution) => void
}

export function ExecutionsDrawer({
  open,
  onOpenChange,
  executions,
  onExecutionSelect,
}: ExecutionsDrawerProps) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className='w-[400px] sm:w-[540px]'>
        <SheetHeader>
          <SheetTitle>{t('requests.executionsDrawer.title')}</SheetTitle>
        </SheetHeader>
        
        <div className='mt-6'>
          {executions.length > 0 ? (
            <ScrollArea className='h-[calc(100vh-120px)]'>
              <div className='space-y-3'>
                {executions.map((execution, index) => (
                  <div
                    key={execution.id}
                    className='rounded-lg border p-4 hover:bg-gray-50 transition-colors'
                  >
                    <div className='flex items-center justify-between mb-3'>
                      <h4 className='text-sm font-medium'>
                        {t('requests.dialogs.requestDetail.execution', { index: index + 1 })}
                      </h4>
                      <div className='flex items-center gap-2'>
                        <Badge className={getStatusColor(execution.status)}>
                          {t(`requests.status.${execution.status}`)}
                        </Badge>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => onExecutionSelect(execution)}
                        >
                          <Eye className='mr-2 h-4 w-4' />
                          {t('requests.actions.viewDetails')}
                        </Button>
                      </div>
                    </div>
                    
                    <div className='space-y-2 text-xs text-muted-foreground'>
                      <div className='flex justify-between'>
                        <span>{t('requests.dialogs.executionDetail.channel')}:</span>
                        <span className='font-mono'>{execution.channel?.name || t('requests.columns.unknown')}</span>
                      </div>
                      <div className='flex justify-between'>
                        <span>{t('requests.dialogs.executionDetail.modelId')}:</span>
                        <span className='font-mono'>{execution.modelID || t('requests.columns.unknown')}</span>
                      </div>
                      <div className='flex justify-between'>
                        <span>{t('requests.dialogs.executionDetail.createdAt')}:</span>
                        <span>
                          {format(
                            new Date(execution.createdAt),
                            'MM-dd HH:mm:ss',
                            { locale }
                          )}
                        </span>
                      </div>
                      {execution.errorMessage && (
                        <div className='mt-2 p-2 bg-red-50 rounded text-red-800'>
                          <div className='font-medium'>{t('requests.dialogs.executionDetail.errorMessage')}:</div>
                          <div className='truncate'>{execution.errorMessage}</div>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <div className='py-8 text-center'>
              <p className='text-muted-foreground text-sm'>{t('requests.dialogs.requestDetail.noExecutions')}</p>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}