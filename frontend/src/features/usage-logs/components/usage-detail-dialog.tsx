import { format } from 'date-fns'
import { zhCN, enUS } from 'date-fns/locale'
import { useTranslation } from 'react-i18next'
import { Database } from 'lucide-react'
import { extractNumberID } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Spinner } from '@/components/spinner'
import { useUsageLog } from '../data/usage-logs'
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions'
import { UsageLogSource } from '../data/schema'

interface UsageDetailDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  usageLogId?: string
}

// Source color mapping for badges
const getSourceColor = (source: UsageLogSource) => {
  switch (source) {
    case 'api':
      return 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800'
    case 'playground':
      return 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800'
    case 'test':
      return 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800'
    default:
      return 'bg-gray-100 text-gray-800 border-gray-200 dark:bg-gray-900/20 dark:text-gray-300 dark:border-gray-800'
  }
}

export function UsageDetailDialog({
  open,
  onOpenChange,
  usageLogId,
}: UsageDetailDialogProps) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const permissions = useUsageLogPermissions()
  const { data: usageLog, isLoading: usageLoading } = useUsageLog(usageLogId!)

  if (!usageLogId) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] max-w-4xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Database className='h-5 w-5' />
            {t('usageLogs.dialogs.usageDetail.title')} - #{extractNumberID(usageLogId)}
          </DialogTitle>
        </DialogHeader>

        {usageLoading ? (
          <div className='flex h-64 items-center justify-center'>
            <Spinner />
          </div>
        ) : !usageLog ? (
          <div className='flex h-64 items-center justify-center'>
            <p className='text-muted-foreground'>
              {t('usageLogs.dialogs.usageDetail.notFound')}
            </p>
          </div>
        ) : (
          <Tabs defaultValue='overview' className='w-full'>
            <TabsList className='grid w-full grid-cols-3'>
              <TabsTrigger value='overview'>
                {t('usageLogs.dialogs.usageDetail.tabs.overview')}
              </TabsTrigger>
              <TabsTrigger value='tokens'>
                {t('usageLogs.dialogs.usageDetail.tabs.tokens')}
              </TabsTrigger>
              <TabsTrigger value='advanced'>
                {t('usageLogs.dialogs.usageDetail.tabs.advanced')}
              </TabsTrigger>
            </TabsList>

            <TabsContent value='overview' className='space-y-4'>
              <div className='grid grid-cols-2 gap-4'>
                {permissions.canViewUsers && usageLog.user && (
                  <div className='space-y-2'>
                    <div className='flex items-center gap-2'>
                      <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.userId')}</span>
                    </div>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.user.firstName && usageLog.user.lastName
                        ? `${usageLog.user.firstName} ${usageLog.user.lastName}`
                        : usageLog.user.email || t('usageLogs.columns.unknown')
                      }
                    </p>
                  </div>
                )}
                
                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.requestId')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm font-mono'>
                    #{extractNumberID(usageLog.requestID)}
                  </p>
                </div>

                {permissions.canViewChannels && usageLog.channel && (
                  <div className='space-y-2'>
                    <div className='flex items-center gap-2'>
                      <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.channelId')}</span>
                    </div>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.channel.name || t('usageLogs.columns.unknown')}
                    </p>
                  </div>
                )}

                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.modelId')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm font-mono'>
                    {usageLog.modelID}
                  </p>
                </div>

                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.source')}</span>
                  </div>
                  <Badge className={getSourceColor(usageLog.source)}>
                    {t(`usageLogs.source.${usageLog.source}`)}
                  </Badge>
                </div>

                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.format')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm'>
                    {usageLog.format}
                  </p>
                </div>

                <div className='space-y-2'>
                  <div className='flex items-center gap-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.fields.createdAt')}</span>
                  </div>
                  <p className='text-muted-foreground text-sm'>
                    {format(
                      new Date(usageLog.createdAt),
                      'yyyy-MM-dd HH:mm:ss',
                      { locale }
                    )}
                  </p>
                </div>
              </div>
            </TabsContent>

            <TabsContent value='tokens' className='space-y-4'>
              <div className='grid grid-cols-1 gap-6'>
                <div className='space-y-4'>
                  <h4 className='text-sm font-medium'>{t('usageLogs.dialogs.usageDetail.tokenDetails.basic')}</h4>
                  <div className='grid grid-cols-3 gap-4'>
                    <div className='text-center p-4 border rounded-lg'>
                      <div className='text-2xl font-bold text-blue-600 dark:text-blue-400'>
                        {usageLog.promptTokens.toLocaleString()}
                      </div>
                      <div className='text-sm text-muted-foreground'>
                        {t('usageLogs.columns.promptTokens')}
                      </div>
                    </div>
                    <div className='text-center p-4 border rounded-lg'>
                      <div className='text-2xl font-bold text-green-600 dark:text-green-400'>
                        {usageLog.completionTokens.toLocaleString()}
                      </div>
                      <div className='text-sm text-muted-foreground'>
                        {t('usageLogs.columns.completionTokens')}
                      </div>
                    </div>
                    <div className='text-center p-4 border rounded-lg'>
                      <div className='text-2xl font-bold text-purple-600 dark:text-purple-400'>
                        {usageLog.totalTokens.toLocaleString()}
                      </div>
                      <div className='text-sm text-muted-foreground'>
                        {t('usageLogs.columns.totalTokens')}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </TabsContent>

            <TabsContent value='advanced' className='space-y-4'>
              <div className='grid grid-cols-2 gap-4'>
                {usageLog.promptAudioTokens !== null && usageLog.promptAudioTokens !== undefined && (
                  <div className='space-y-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.columns.promptAudioTokens')}</span>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.promptAudioTokens.toLocaleString()}
                    </p>
                  </div>
                )}

                {usageLog.promptCachedTokens !== null && usageLog.promptCachedTokens !== undefined && (
                  <div className='space-y-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.columns.promptCachedTokens')}</span>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.promptCachedTokens.toLocaleString()}
                    </p>
                  </div>
                )}

                {usageLog.completionAudioTokens !== null && usageLog.completionAudioTokens !== undefined && (
                  <div className='space-y-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.columns.completionAudioTokens')}</span>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.completionAudioTokens.toLocaleString()}
                    </p>
                  </div>
                )}

                {usageLog.completionReasoningTokens !== null && usageLog.completionReasoningTokens !== undefined && (
                  <div className='space-y-2'>
                    <span className='text-sm font-medium'>{t('usageLogs.columns.completionReasoningTokens')}</span>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.completionReasoningTokens.toLocaleString()}
                    </p>
                  </div>
                )}

                {usageLog.completionAcceptedPredictionTokens !== null && usageLog.completionAcceptedPredictionTokens !== undefined && (
                  <div className='space-y-2'>
                    <span className='text-sm font-medium'>Accepted Prediction Tokens</span>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.completionAcceptedPredictionTokens.toLocaleString()}
                    </p>
                  </div>
                )}

                {usageLog.completionRejectedPredictionTokens !== null && usageLog.completionRejectedPredictionTokens !== undefined && (
                  <div className='space-y-2'>
                    <span className='text-sm font-medium'>Rejected Prediction Tokens</span>
                    <p className='text-muted-foreground text-sm'>
                      {usageLog.completionRejectedPredictionTokens.toLocaleString()}
                    </p>
                  </div>
                )}
              </div>
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  )
}