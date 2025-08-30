import { format } from 'date-fns'
import { useParams, useNavigate } from '@tanstack/react-router'
import { zhCN, enUS } from 'date-fns/locale'
import { Copy, Clock, User, Key, Database, ArrowLeft, FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { extractNumberID } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { LanguageSwitch } from '@/components/language-switch'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { JsonViewer } from '@/components/json-tree-view'
import { useRequest, useRequestExecutions } from '../data'
import { useUsageLogs } from '../../usage-logs/data/usage-logs'
import { getStatusColor } from './help'

export default function RequestDetailPage() {
  const { t, i18n } = useTranslation()
  const { requestId } = useParams({ from: '/_authenticated/requests/$requestId' })
  const navigate = useNavigate()
  const locale = i18n.language === 'zh' ? zhCN : enUS

  const { data: request, isLoading } = useRequest(requestId)
  const { data: executions } = useRequestExecutions(requestId, {
    orderBy: { field: 'CREATED_AT', direction: 'DESC' }
  })
  const { data: usageLogs } = useUsageLogs({
    where: { requestID: requestId },
    orderBy: { field: 'CREATED_AT', direction: 'DESC' }
  })

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success(t('requests.actions.copy'))
  }

  const formatJson = (data: any) => {
    if (!data) return ''
    try {
      return JSON.stringify(data, null, 2)
    } catch {
      return String(data)
    }
  }

  const handleBack = () => {
    navigate({ to: '/requests' })
  }

  if (isLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'>
          <div className='ml-auto flex items-center space-x-4'>
            <ThemeSwitch />
            <LanguageSwitch />
            <ProfileDropdown />
          </div>
        </Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2'></div>
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
          </div>
        </Main>
      </div>
    )
  }

  if (!request) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'>
          <div className='ml-auto flex items-center space-x-4'>
            <ThemeSwitch />
            <LanguageSwitch />
            <ProfileDropdown />
          </div>
        </Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-6 text-center'>
              <div className='space-y-2'>
                <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                <p className='text-muted-foreground text-xl font-medium'>
                  {t('requests.dialogs.requestDetail.notFound')}
                </p>
              </div>
              <Button onClick={handleBack} size='lg'>
                <ArrowLeft className='mr-2 h-4 w-4' />
                {t('common.back')}
              </Button>
            </div>
          </div>
        </Main>
      </div>
    )
  }

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center space-x-4'>
          <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
            <ArrowLeft className='mr-2 h-4 w-4' />
            {t('common.back')}
          </Button>
          <Separator orientation='vertical' className='h-6' />
          <div className='flex items-center space-x-3'>
            <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
              <FileText className='text-primary h-4 w-4' />
            </div>
            <div>
              <h1 className='text-lg leading-none font-semibold'>
                {t('requests.detail.title')} #{extractNumberID(request.id) || request.id}
              </h1>
              <p className='text-muted-foreground mt-1 text-sm'>{request.modelID || t('requests.columns.unknown')}</p>
            </div>
          </div>
        </div>
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <LanguageSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main className='flex-1 overflow-auto'>
        <div className='container mx-auto max-w-7xl space-y-8 p-6'>
          {/* Request Overview Card */}
          <Card className='border-0 shadow-sm'>
            <CardHeader className='pb-4'>
              <CardTitle className='flex items-center justify-between'>
                <div className='flex items-center gap-3'>
                  <div className='bg-primary/10 flex h-10 w-10 items-center justify-center rounded-lg'>
                    <Database className='text-primary h-5 w-5' />
                  </div>
                  <span className='text-xl'>{t('requests.detail.overview')}</span>
                </div>
                <Badge className={getStatusColor(request.status)} variant='secondary'>
                  {t(`requests.status.${request.status}`)}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4'>
                <div className='bg-muted/30 space-y-3 rounded-lg border p-4'>
                  <div className='flex items-center gap-2'>
                    <Database className='text-primary h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.columns.modelId')}</span>
                  </div>
                  <p className='bg-background rounded border px-3 py-2 font-mono text-sm'>
                    {request.modelID || t('requests.columns.unknown')}
                  </p>
                </div>

                <div className='bg-muted/30 space-y-3 rounded-lg border p-4'>
                  <div className='flex items-center gap-2'>
                    <User className='text-primary h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.dialogs.requestDetail.fields.userId')}</span>
                  </div>
                  <p className='text-muted-foreground font-mono text-sm'>
                    {request.user
                      ? `${request.user.firstName || ''} ${request.user.lastName || ''}`.trim() ||
                        t('requests.columns.unknown')
                      : t('requests.columns.unknown')}
                  </p>
                </div>

                <div className='bg-muted/30 space-y-3 rounded-lg border p-4'>
                  <div className='flex items-center gap-2'>
                    <Key className='text-primary h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.dialogs.requestDetail.fields.apiKeyId')}</span>
                  </div>
                  <p className='text-muted-foreground font-mono text-sm'>
                    {request.apiKey?.name || t('requests.columns.unknown')}
                  </p>
                </div>

                <div className='bg-muted/30 space-y-3 rounded-lg border p-4'>
                  <div className='flex items-center gap-2'>
                    <Clock className='text-primary h-4 w-4' />
                    <span className='text-sm font-medium'>{t('requests.columns.createdAt')}</span>
                  </div>
                  <p className='text-muted-foreground font-mono text-sm'>
                    {format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Request and Response Tabs */}
          <Card className='border-0 shadow-sm'>
            <CardContent className='p-0'>
              <Tabs defaultValue='request' className='w-full'>
                <div className='bg-muted/20 border-b px-6 pt-6'>
                  <TabsList className='bg-background grid w-full grid-cols-4'>
                    <TabsTrigger
                      value='request'
                      className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'
                    >
                      {t('requests.detail.tabs.request')}
                    </TabsTrigger>
                    <TabsTrigger
                      value='response'
                      className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'
                    >
                      {t('requests.detail.tabs.response')}
                    </TabsTrigger>
                    <TabsTrigger
                      value='executions'
                      className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'
                    >
                      {t('requests.detail.tabs.executions')}
                    </TabsTrigger>
                    <TabsTrigger
                      value='usage'
                      className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'
                    >
                      {t('requests.detail.tabs.usage')}
                    </TabsTrigger>
                  </TabsList>
                </div>

                <TabsContent value='request' className='space-y-6 p-6'>
                  <div className='space-y-4'>
                    <div className='flex items-center justify-between'>
                      <h4 className='flex items-center gap-2 text-base font-semibold'>
                        <FileText className='text-primary h-4 w-4' />
                        {t('requests.columns.requestBody')}
                      </h4>
                      <Button
                        variant='outline'
                        size='sm'
                        onClick={() => copyToClipboard(formatJson(request.requestBody))}
                        className='hover:bg-primary hover:text-primary-foreground'
                      >
                        <Copy className='mr-2 h-4 w-4' />
                        {t('requests.dialogs.jsonViewer.copy')}
                      </Button>
                    </div>
                    <div className='bg-muted/20 h-[500px] w-full rounded-lg border overflow-auto p-4'>
                      <JsonViewer 
                        data={request.requestBody} 
                        rootName="" 
                        defaultExpanded={true}
                        className="text-sm"
                      />
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value='response' className='space-y-6 p-6'>
                  <div className='space-y-4'>
                    <div className='flex items-center justify-between'>
                      <h4 className='flex items-center gap-2 text-base font-semibold'>
                        <FileText className='text-primary h-4 w-4' />
                        {t('requests.columns.responseBody')}
                      </h4>
                      <Button
                        variant='outline'
                        size='sm'
                        onClick={() => copyToClipboard(formatJson(request.responseBody))}
                        disabled={!request.responseBody}
                        className='hover:bg-primary hover:text-primary-foreground disabled:opacity-50'
                      >
                        <Copy className='mr-2 h-4 w-4' />
                        {t('requests.dialogs.jsonViewer.copy')}
                      </Button>
                    </div>
                    {request.responseBody ? (
                      <div className='bg-muted/20 h-[500px] w-full rounded-lg border overflow-auto p-4'>
                        <JsonViewer 
                          data={request.responseBody} 
                          rootName="" 
                          defaultExpanded={true}
                          className="text-sm"
                        />
                      </div>
                    ) : (
                      <div className='bg-muted/20 flex h-[500px] w-full items-center justify-center rounded-lg border'>
                        <div className='space-y-3 text-center'>
                          <FileText className='text-muted-foreground mx-auto h-12 w-12' />
                          <p className='text-muted-foreground text-base'>{t('requests.detail.noResponse')}</p>
                        </div>
                      </div>
                    )}
                  </div>
                </TabsContent>

                <TabsContent value='executions' className='space-y-6 p-6'>
                  {executions && executions.edges.length > 0 ? (
                    <div className='space-y-6'>
                      {executions.edges.map((edge: any, index: number) => {
                        const execution = edge.node
                        return (
                          <Card key={execution.id} className='bg-muted/20 border-0 shadow-sm'>
                            <CardHeader className='pb-4'>
                              <div className='flex items-center justify-between'>
                                <h5 className='flex items-center gap-2 text-base font-semibold'>
                                  <div className='bg-primary/10 text-primary flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold'>
                                    {index + 1}
                                  </div>
                                  {t('requests.dialogs.requestDetail.execution', { index: index + 1 })}
                                </h5>
                                <Badge className={getStatusColor(execution.status)} variant='secondary'>
                                  {t(`requests.status.${execution.status}`)}
                                </Badge>
                              </div>
                            </CardHeader>
                            <CardContent className='space-y-6'>
                              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='flex items-center gap-2 text-sm font-medium'>
                                    <Clock className='text-primary h-4 w-4' />
                                    {t('requests.dialogs.requestDetail.fields.startTime')}
                                  </span>
                                  <p className='text-muted-foreground font-mono text-sm'>
                                    {execution.createdAt
                                      ? format(new Date(execution.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })
                                      : t('requests.columns.unknown')}
                                  </p>
                                </div>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='flex items-center gap-2 text-sm font-medium'>
                                    <Clock className='text-primary h-4 w-4' />
                                    {t('requests.dialogs.requestDetail.fields.endTime')}
                                  </span>
                                  <p className='text-muted-foreground font-mono text-sm'>
                                    {execution.updatedAt
                                      ? format(new Date(execution.updatedAt), 'yyyy-MM-dd HH:mm:ss', { locale })
                                      : t('requests.columns.unknown')}
                                  </p>
                                </div>
                              </div>

                              {execution.errorMessage && (
                                <div className='bg-destructive/5 border-destructive/20 space-y-3 rounded-lg border p-4'>
                                  <span className='text-destructive flex items-center gap-2 text-sm font-semibold'>
                                    <FileText className='h-4 w-4' />
                                    {t('common.errorMessage')}
                                  </span>
                                  <p className='text-destructive bg-destructive/10 rounded border p-3 text-sm'>
                                    {execution.errorMessage}
                                  </p>
                                </div>
                              )}

                              {execution.requestBody && (
                                <div className='space-y-3'>
                                  <div className='flex items-center justify-between'>
                                    <span className='flex items-center gap-2 text-sm font-semibold'>
                                      <FileText className='text-primary h-4 w-4' />
                                      {t('requests.columns.requestBody')}
                                    </span>
                                    <Button
                                      variant='outline'
                                      size='sm'
                                      onClick={() => copyToClipboard(formatJson(execution.requestBody))}
                                      className='hover:bg-primary hover:text-primary-foreground'
                                    >
                                      <Copy className='mr-2 h-4 w-4' />
                                      {t('requests.dialogs.jsonViewer.copy')}
                                    </Button>
                                  </div>
                                  <div className='bg-background h-64 w-full rounded-lg border overflow-auto p-3'>
                                    <JsonViewer 
                                      data={execution.requestBody} 
                                      rootName="" 
                                      defaultExpanded={false}
                                      className="text-xs"
                                    />
                                  </div>
                                </div>
                              )}

                              {execution.responseBody && (
                                <div className='space-y-3'>
                                  <div className='flex items-center justify-between'>
                                    <span className='flex items-center gap-2 text-sm font-semibold'>
                                      <FileText className='text-primary h-4 w-4' />
                                      {t('requests.columns.responseBody')}
                                    </span>
                                    <Button
                                      variant='outline'
                                      size='sm'
                                      onClick={() => copyToClipboard(formatJson(execution.responseBody))}
                                      className='hover:bg-primary hover:text-primary-foreground'
                                    >
                                      <Copy className='mr-2 h-4 w-4' />
                                      {t('requests.dialogs.jsonViewer.copy')}
                                    </Button>
                                  </div>
                                  <div className='bg-background h-64 w-full rounded-lg border overflow-auto p-3'>
                                    <JsonViewer 
                                      data={execution.responseBody} 
                                      rootName="" 
                                      defaultExpanded={false}
                                      className="text-xs"
                                    />
                                  </div>
                                </div>
                              )}
                            </CardContent>
                          </Card>
                        )
                      })}
                    </div>
                  ) : (
                    <div className='py-16 text-center'>
                      <div className='space-y-4'>
                        <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                        <p className='text-muted-foreground text-lg'>
                          {t('requests.dialogs.requestDetail.noExecutions')}
                        </p>
                      </div>
                    </div>
                  )}
                </TabsContent>

                <TabsContent value='usage' className='space-y-6 p-6'>
                  {usageLogs && usageLogs.edges.length > 0 ? (
                    <div className='space-y-6'>
                      {usageLogs.edges.map((edge: any, index: number) => {
                        const usage = edge.node
                        const cachedTokens = usage.promptCachedTokens || 0
                        const hasCache = cachedTokens > 0
                        const cacheHitRate = hasCache ? ((cachedTokens / usage.promptTokens) * 100).toFixed(1) : 0
                        
                        return (
                          <Card key={usage.id} className='bg-muted/20 border-0 shadow-sm'>
                            <CardHeader className='pb-4'>
                              <div className='flex items-center justify-between'>
                                <h5 className='flex items-center gap-2 text-base font-semibold'>
                                  <div className='bg-primary/10 text-primary flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold'>
                                    {index + 1}
                                  </div>
                                  {t('requests.detail.tabs.usage', { index: index + 1 })}
                                </h5>
                                <Badge className='bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300' variant='secondary'>
                                  {t(`usageLogs.source.${usage.source}`)}
                                </Badge>
                              </div>
                            </CardHeader>
                            <CardContent className='space-y-6'>
                              {/* Token Usage Grid */}
                              <div className='grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4'>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='text-xs text-muted-foreground font-medium'>
                                    {t('usageLogs.columns.inputLabel')}
                                  </span>
                                  <p className='text-lg font-semibold text-blue-600 dark:text-blue-400'>
                                    {usage.promptTokens.toLocaleString()}
                                  </p>
                                </div>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='text-xs text-muted-foreground font-medium'>
                                    {t('usageLogs.columns.outputLabel')}
                                  </span>
                                  <p className='text-lg font-semibold text-green-600 dark:text-green-400'>
                                    {usage.completionTokens.toLocaleString()}
                                  </p>
                                </div>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='text-xs text-muted-foreground font-medium'>
                                    {t('usageLogs.columns.totalTokens')}
                                  </span>
                                  <p className='text-lg font-semibold text-slate-600 dark:text-slate-400'>
                                    {usage.totalTokens.toLocaleString()}
                                  </p>
                                </div>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='text-xs text-muted-foreground font-medium'>
                                    {t('usageLogs.columns.cacheTokens')}
                                  </span>
                                  <div className='space-y-1'>
                                    <div className='flex items-center gap-1'>
                                      <span className={`text-xs font-medium ${
                                        hasCache 
                                          ? 'text-green-600 dark:text-green-400' 
                                          : 'text-muted-foreground'
                                      }`}>
                                        {hasCache ? '✓' : '—'}
                                      </span>
                                      <span className='text-lg font-semibold text-purple-600 dark:text-purple-400'>
                                        {cachedTokens.toLocaleString()}
                                      </span>
                                    </div>
                                    {hasCache && (
                                      <div className='text-xs text-muted-foreground'>
                                        {t('usageLogs.columns.cacheHitRate', { rate: cacheHitRate })}
                                      </div>
                                    )}
                                  </div>
                                </div>
                              </div>

                              {/* Additional Token Types */}
                              {(usage.completionReasoningTokens > 0 || usage.promptAudioTokens > 0 || usage.completionAudioTokens > 0) && (
                                <div className='grid grid-cols-2 gap-4 sm:grid-cols-3'>
                                  {usage.completionReasoningTokens > 0 && (
                                    <div className='bg-background space-y-2 rounded-lg border p-3'>
                                      <span className='text-xs text-muted-foreground font-medium'>
                                        {t('requests.detail.usage.reasoningTokens')}
                                      </span>
                                      <p className='text-lg font-semibold text-orange-600 dark:text-orange-400'>
                                        {usage.completionReasoningTokens.toLocaleString()}
                                      </p>
                                    </div>
                                  )}
                                  {usage.promptAudioTokens > 0 && (
                                    <div className='bg-background space-y-2 rounded-lg border p-3'>
                                      <span className='text-xs text-muted-foreground font-medium'>
                                        Audio Input
                                      </span>
                                      <p className='text-lg font-semibold text-cyan-600 dark:text-cyan-400'>
                                        {usage.promptAudioTokens.toLocaleString()}
                                      </p>
                                    </div>
                                  )}
                                  {usage.completionAudioTokens > 0 && (
                                    <div className='bg-background space-y-2 rounded-lg border p-3'>
                                      <span className='text-xs text-muted-foreground font-medium'>
                                        Audio Output
                                      </span>
                                      <p className='text-lg font-semibold text-teal-600 dark:text-teal-400'>
                                        {usage.completionAudioTokens.toLocaleString()}
                                      </p>
                                    </div>
                                  )}
                                </div>
                              )}
                            </CardContent>
                          </Card>
                        )
                      })}
                    </div>
                  ) : (
                    <div className='py-16 text-center'>
                      <div className='space-y-4'>
                        <Database className='text-muted-foreground mx-auto h-16 w-16' />
                        <p className='text-muted-foreground text-lg'>
                          {t('requests.detail.usage.noUsage')}
                        </p>
                      </div>
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </div>
      </Main>
    </div>
  )
}
