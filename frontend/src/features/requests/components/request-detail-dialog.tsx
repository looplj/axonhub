import { format } from 'date-fns'
import { zhCN, enUS } from 'date-fns/locale'
import { useTranslation } from 'react-i18next'
import { Copy, Clock, User, Key, Database } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useRequest, useRequestExecutions } from '../data'
import { getStatusColor } from './help'

interface RequestDetailDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  requestId?: string
}

export function RequestDetailDialog({
  open,
  onOpenChange,
  requestId,
}: RequestDetailDialogProps) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const { data: request, isLoading: requestLoading } = useRequest(requestId!)

  const { data: executionsData, isLoading: executionsLoading } =
    useRequestExecutions(requestId!, {
      first: 50,
    })

  const executions = executionsData?.edges?.map((edge) => edge.node) || []

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  const formatJson = (jsonString: string) => {
    try {
      return JSON.stringify(JSON.parse(jsonString), null, 2)
    } catch {
      return jsonString
    }
  }

  if (!requestId) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] max-w-4xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Database className='h-5 w-5' />
            {t('requests.dialogs.requestDetail.title')} - {requestId}
          </DialogTitle>
        </DialogHeader>

        {requestLoading ? (
          <div className='flex h-64 items-center justify-center'>
            <div className='text-center'>
              <div className='mx-auto mb-2 h-8 w-8 animate-spin rounded-full border-b-2 border-gray-900'></div>
              <p className='text-muted-foreground text-sm'>{t('loading')}</p>
            </div>
          </div>
        ) : request ? (
          <Tabs defaultValue='overview' className='w-full'>
            <TabsList className='grid w-full grid-cols-4'>
              <TabsTrigger value='overview'>{t('requests.dialogs.requestDetail.tabs.overview')}</TabsTrigger>
              <TabsTrigger value='request'>{t('requests.dialogs.requestDetail.tabs.request')}</TabsTrigger>
              <TabsTrigger value='response'>{t('requests.dialogs.requestDetail.tabs.response')}</TabsTrigger>
              <TabsTrigger value='executions'>{t('requests.dialogs.requestDetail.tabs.executions')}</TabsTrigger>
            </TabsList>

            <TabsContent value='overview' className='space-y-4'>
              <div className='grid grid-cols-2 gap-4'>
                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <User className='text-muted-foreground h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.dialogs.requestDetail.fields.userId')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm'>
                    {request.user?.id || t('requests.columns.unknown')}
                  </p>
                </div>
                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <Key className='text-muted-foreground h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.dialogs.requestDetail.fields.apiKeyId')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm'>
                    {request.apiKey?.id || t('requests.columns.unknown')}
                  </p>
                </div>
                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <Clock className='text-muted-foreground h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.columns.createdAt')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm'>
                    {format(
                      new Date(request.createdAt),
                      'yyyy-MM-dd HH:mm:ss',
                      { locale }
                    )}
                  </p>
                </div>
                <div className='space-y-2'>
                  <span className='text-sm font-medium'>{t('requests.columns.status')}</span>
                  <Badge className={getStatusColor(request.status)}>
                    {t(`requests.status.${request.status}`)}
                  </Badge>
                </div>
              </div>
            </TabsContent>

            <TabsContent value='request' className='space-y-4'>
              <div className='space-y-2'>
                <div className='flex items-center justify-between'>
                  <h4 className='text-sm font-medium'>{t('requests.columns.requestBody')}</h4>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      copyToClipboard(formatJson(request.requestBody))
                    }
                  >
                    <Copy className='mr-2 h-4 w-4' />
                    {t('requests.dialogs.jsonViewer.copy')}
                  </Button>
                </div>
                <ScrollArea className='h-64 w-full rounded-md border p-4'>
                  <pre className='text-xs whitespace-pre-wrap'>
                    {formatJson(request.requestBody)}
                  </pre>
                </ScrollArea>
              </div>
            </TabsContent>

            <TabsContent value='response' className='space-y-4'>
              <div className='space-y-2'>
                <div className='flex items-center justify-between'>
                  <h4 className='text-sm font-medium'>{t('requests.columns.responseBody')}</h4>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      copyToClipboard(formatJson(request.responseBody || ''))
                    }
                    disabled={!request.responseBody}
                  >
                    <Copy className='mr-2 h-4 w-4' />
                    {t('requests.dialogs.jsonViewer.copy')}
                  </Button>
                </div>
                <ScrollArea className='h-64 w-full rounded-md border p-4'>
                  <pre className='text-xs whitespace-pre-wrap'>
                    {request.responseBody
                      ? formatJson(request.responseBody)
                      : t('requests.dialogs.executionDetail.noResponse')}
                  </pre>
                </ScrollArea>
              </div>
            </TabsContent>

            <TabsContent value='executions' className='space-y-4'>
              {executionsLoading ? (
                <div className='flex h-32 items-center justify-center'>
                  <div className='h-6 w-6 animate-spin rounded-full border-b-2 border-gray-900'></div>
                </div>
              ) : executions.length > 0 ? (
                <div className='space-y-4'>
                  {executions.map((execution, index) => (
                    <div
                      key={execution.id}
                      className='space-y-3 rounded-lg border p-4'
                    >
                      <div className='flex items-center justify-between'>
                        <h5 className='text-sm font-medium'>
                          {t('requests.dialogs.requestDetail.execution', { index: index + 1 })}
                        </h5>
                        <Badge className={getStatusColor(execution.status)}>
                          {t(`requests.status.${execution.status}`)}
                        </Badge>
                      </div>

                      <div className='grid grid-cols-2 gap-4 text-xs'>
                        <div>
                          <span className='font-medium'>{t('requests.dialogs.requestDetail.fields.startTime')}:</span>
                          <p className='text-muted-foreground'>
                            {execution.createdAt
                              ? format(
                                  new Date(execution.createdAt),
                                  'yyyy-MM-dd HH:mm:ss',
                                  { locale }
                                )
                              : t('requests.dialogs.requestDetail.fields.notStarted')}
                          </p>
                        </div>
                        <div>
                          <span className='font-medium'>{t('requests.dialogs.requestDetail.fields.endTime')}:</span>
                          {/* <p className='text-muted-foreground'>
                            {execution.finishedAt
                              ? format(
                                  new Date(execution.finishedAt),
                                  'yyyy-MM-dd HH:mm:ss',
                                  { locale: zhCN }
                                )
                              : '未结束'}
                          </p> */}
                        </div>
                      </div>

                      {execution.errorMessage && (
                        <div className='space-y-1'>
                          <span className='text-xs font-medium text-red-600'>
                            {t('requests.dialogs.executionDetail.errorMessage')}:
                          </span>
                          <ScrollArea className='h-20 w-full rounded border bg-red-50 p-2'>
                            <pre className='text-xs whitespace-pre-wrap text-red-800'>
                              {execution.errorMessage}
                            </pre>
                          </ScrollArea>
                        </div>
                      )}

                      {/* {execution.metadata && (
                        <div className='space-y-1'>
                          <div className='flex items-center justify-between'>
                            <span className='text-xs font-medium'>元数据:</span>
                            <Button
                              variant='ghost'
                              size='sm'
                              onClick={() =>
                                copyToClipboard(formatJson(execution.metadata!))
                              }
                            >
                              <Copy className='h-3 w-3' />
                            </Button>
                          </div>
                          <ScrollArea className='h-20 w-full rounded border bg-gray-50 p-2'>
                            <pre className='text-xs whitespace-pre-wrap'>
                              {formatJson(execution.metadata)}
                            </pre>
                          </ScrollArea>
                        </div>
                      )} */}
                    </div>
                  ))}
                </div>
              ) : (
                <div className='py-8 text-center'>
                  <p className='text-muted-foreground text-sm'>{t('requests.dialogs.requestDetail.noExecutions')}</p>
                </div>
              )}
            </TabsContent>
          </Tabs>
        ) : (
          <div className='py-8 text-center'>
            <p className='text-muted-foreground text-sm'>{t('requests.dialogs.requestDetail.notFound')}</p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
