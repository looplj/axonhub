import { useState, useCallback } from 'react';
import { format } from 'date-fns';
import { DashboardIcon } from '@radix-ui/react-icons';
import { useParams, useNavigate } from '@tanstack/react-router';
import { zhCN, enUS } from 'date-fns/locale';
import { Copy, Clock, Key, Database, ArrowLeft, FileText, Layers, Download } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { JsonViewer } from '@/components/json-tree-view';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useGeneralSettings } from '@/features/system/data/system';
import { useUsageLogs } from '../../usage-logs/data/usage-logs';
import { useRequest, useRequestExecutions } from '../data';
import { ChunksDialog } from './chunks-dialog';
import { getStatusColor } from './help';

export default function RequestDetailPage() {
  const { t, i18n } = useTranslation();
  const { requestId } = useParams({ from: '/_authenticated/project/requests/$requestId' });
  const navigate = useNavigate();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const { getSearchParams } = usePaginationSearch({ defaultPageSize: 20 });

  const [showResponseChunks, setShowResponseChunks] = useState(false);
  const [showExecutionChunks, setShowExecutionChunks] = useState(false);
  const [selectedResponseChunks, setSelectedResponseChunks] = useState<any[]>([]);
  const [selectedExecutionChunks, setSelectedExecutionChunks] = useState<any[]>([]);

  const { data: settings } = useGeneralSettings();
  const { data: request, isLoading } = useRequest(requestId);
  const { data: executions } = useRequestExecutions(requestId, {
    first: 10,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
  });
  const { data: usageLogs } = useUsageLogs({
    first: 1,
    where: { requestID: requestId },
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
  });

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success(t('requests.actions.copy'));
  };

  const downloadFile = (content: string, filename: string) => {
    const blob = new Blob([content], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success(t('requests.actions.download'));
  };

  const showResponseChunksModal = useCallback(() => {
    if (request?.responseChunks) {
      setSelectedResponseChunks(request.responseChunks);
      setShowResponseChunks(true);
    }
  }, [request]);

  const showExecutionChunksModal = useCallback((chunks: any[]) => {
    if (chunks && chunks.length > 0) {
      setSelectedExecutionChunks(chunks);
      setShowExecutionChunks(true);
    }
  }, []);

  const formatJson = (data: any) => {
    if (!data) return '';
    try {
      return JSON.stringify(data, null, 2);
    } catch {
      return String(data);
    }
  };

  const calculateLatency = (createdAt: string | Date, updatedAt: string | Date) => {
    if (!createdAt || !updatedAt) return null;
    const start = new Date(createdAt).getTime();
    const end = new Date(updatedAt).getTime();
    const diffMs = end - start;
    if (diffMs < 0) return null;
    return diffMs;
  };

  const formatLatency = (latencyMs: number | null) => {
    if (latencyMs === null) return t('requests.columns.unknown');
    if (latencyMs < 1000) return `${latencyMs}ms`;
    return `${(latencyMs / 1000).toFixed(2)}s`;
  };

  const handleBack = () => {
    // 保持分页状态返回到请求列表页
    navigate({
      to: '/project/requests',
      search: getSearchParams(),
    });
  };

  if (isLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'></Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2'></div>
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
          </div>
        </Main>
      </div>
    );
  }

  if (!request) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'></Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-6 text-center'>
              <div className='space-y-2'>
                <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                <p className='text-muted-foreground text-xl font-medium'>{t('requests.dialogs.requestDetail.notFound')}</p>
              </div>
              <Button onClick={handleBack} size='lg'>
                <ArrowLeft className='mr-2 h-4 w-4' />
                {t('common.back')}
              </Button>
            </div>
          </div>
        </Main>
      </div>
    );
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
              <div className='mt-1 flex items-center gap-2'>
                <p className='text-muted-foreground text-sm'>{request.modelID || t('requests.columns.unknown')}</p>
                <span className='text-muted-foreground text-xs'>•</span>
                <p className='text-muted-foreground text-xs'>{format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}</p>
              </div>
            </div>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-auto'>
        <div className='container mx-auto max-w-7xl space-y-8 p-6'>
          {/* Request Overview Card */}
          <Card className='border-0 shadow-sm'>
            <CardHeader className='pb-2'>
              <CardTitle className='flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <div className='bg-primary/10 flex h-7 w-7 items-center justify-center rounded-lg'>
                    <DashboardIcon className='text-primary h-3.5 w-3.5' />
                  </div>
                  <span className='text-base'>{t('requests.detail.overview')}</span>
                </div>
                <Badge className={getStatusColor(request.status)} variant='secondary'>
                  {t(`requests.status.${request.status}`)}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3'>
                <div className='bg-muted/30 flex items-center justify-between gap-2 rounded-lg border px-3 py-2'>
                  <div className='flex items-center gap-2'>
                    <Database className='text-primary h-3.5 w-3.5' />
                    <span className='text-xs font-medium'>{t('requests.columns.channel')}</span>
                  </div>
                  <p className='bg-background rounded border px-2 py-0.5 font-mono text-xs'>
                    {request.channel?.name || t('requests.columns.unknown')}
                  </p>
                </div>

                <div className='bg-muted/30 flex items-center justify-between gap-2 rounded-lg border px-3 py-2'>
                  <div className='flex items-center gap-2'>
                    <Database className='text-primary h-3.5 w-3.5' />
                    <span className='text-xs font-medium'>{t('requests.columns.modelId')}</span>
                  </div>
                  <p className='bg-background rounded border px-2 py-0.5 font-mono text-xs'>
                    {request.modelID || t('requests.columns.unknown')}
                  </p>
                </div>

                <div className='bg-muted/30 flex items-center justify-between gap-2 rounded-lg border px-3 py-2'>
                  <div className='flex items-center gap-2'>
                    <Key className='text-primary h-3.5 w-3.5' />
                    <span className='text-xs font-medium'>{t('requests.dialogs.requestDetail.fields.apiKeyName')}</span>
                  </div>
                  <p className='text-muted-foreground font-mono text-xs'>{request.apiKey?.name || t('requests.columns.unknown')}</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Usage Card */}
          {usageLogs &&
            usageLogs.edges.length > 0 &&
            (() => {
              const usage = usageLogs.edges[0].node;
              const promptTokens = usage.promptTokens || 0;
              const cachedTokens = usage.promptCachedTokens || 0;
              const writeCachedTokens = usage.promptWriteCachedTokens || 0;
              const hasReadCache = cachedTokens > 0;
              const hasWriteCache = writeCachedTokens > 0;
              const cacheHitRate = hasReadCache ? ((cachedTokens / promptTokens) * 100).toFixed(1) : '0.0';
              const writeCacheRate = hasWriteCache ? ((writeCachedTokens / promptTokens) * 100).toFixed(1) : '0.0';
              const cost = usage.totalCost ?? 0;

              const promptCost = usage.costItems?.find((i: any) => i.itemCode === 'prompt_tokens')?.subtotal;
              const completionCost = usage.costItems?.find((i: any) => i.itemCode === 'completion_tokens')?.subtotal;
              const cacheReadCost = usage.costItems?.find((i: any) => i.itemCode === 'prompt_cached_tokens')?.subtotal;
              const cacheWriteCost = usage.costItems?.find((i: any) => i.itemCode === 'prompt_write_cached_tokens')?.subtotal;

              const formatCurrency = (val: number) =>
                t('currencies.format', {
                  val,
                  currency: settings?.currencyCode,
                  locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                  minimumFractionDigits: 6,
                });

              const renderCost = (val: number | null | undefined) => {
                if (cost <= 0) return '-';
                if (val == null || val <= 0) return '-';
                return formatCurrency(val);
              };

              return (
                <Card className='border-0 shadow-sm'>
                  <CardHeader className='pb-2'>
                    <CardTitle className='flex items-center justify-between'>
                      <div className='flex items-center gap-2'>
                        <div className='bg-primary/10 flex h-7 w-7 items-center justify-center rounded-lg'>
                          <Database className='text-primary h-3.5 w-3.5' />
                        </div>
                        <span className='text-base'>{t('requests.detail.tabs.usage')}</span>
                      </div>
                      <Badge className='bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300' variant='secondary'>
                        {t(`usageLogs.source.${usage.source}`)}
                      </Badge>
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className='grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5'>
                      <div className='bg-muted/30 flex flex-col justify-center rounded-lg border px-2.5 py-2'>
                        <span className='text-muted-foreground text-xs font-medium'>{t('usageLogs.columns.inputLabel')}</span>
                        <div className='mt-1'>
                          <p className='text-sm font-semibold'>{usage.promptTokens.toLocaleString()}</p>
                          <p className='text-muted-foreground text-xs'>{renderCost(promptCost)}</p>
                        </div>
                      </div>
                      <div className='bg-muted/30 flex flex-col justify-center rounded-lg border px-2.5 py-2'>
                        <span className='text-muted-foreground text-xs font-medium'>{t('usageLogs.columns.outputLabel')}</span>
                        <div className='mt-1'>
                          <p className='text-sm font-semibold'>{usage.completionTokens.toLocaleString()}</p>
                          <p className='text-muted-foreground text-xs'>{renderCost(completionCost)}</p>
                        </div>
                      </div>

                      <div className='bg-muted/30 flex flex-col justify-center rounded-lg border px-2.5 py-2'>
                        <span className='text-muted-foreground text-xs font-medium'>{t('usageLogs.columns.promptCachedTokens')}</span>
                        <div className='mt-1'>
                          <div className='flex items-center justify-between'>
                            <p className='text-sm font-semibold'>{cachedTokens.toLocaleString()}</p>
                            {hasReadCache && (
                              <Badge variant='outline' className='h-4 px-1 text-[10px] text-green-600 border-green-200 bg-green-50'>
                                {cacheHitRate}%
                              </Badge>
                            )}
                          </div>
                          <p className='text-muted-foreground text-xs'>{renderCost(cacheReadCost)}</p>
                        </div>
                      </div>
                      <div className='bg-muted/30 flex flex-col justify-center rounded-lg border px-2.5 py-2'>
                        <span className='text-muted-foreground text-xs font-medium'>{t('usageLogs.columns.writeCacheTokens')}</span>
                        <div className='mt-1'>
                          <div className='flex items-center justify-between'>
                            <p className='text-sm font-semibold'>{writeCachedTokens.toLocaleString()}</p>
                            {hasWriteCache && (
                              <Badge variant='outline' className='h-4 px-1 text-[10px] text-blue-600 border-blue-200 bg-blue-50'>
                                {writeCacheRate}%
                              </Badge>
                            )}
                          </div>
                          <p className='text-muted-foreground text-xs'>{renderCost(cacheWriteCost)}</p>
                        </div>
                      </div>
                      <div className='bg-muted/30 flex flex-col justify-center rounded-lg border px-2.5 py-2'>
                        <span className='text-muted-foreground text-xs font-medium'>{t('usageLogs.columns.totalTokens')}</span>
                        <div className='mt-1'>
                          <p className='text-sm font-semibold'>{usage.totalTokens.toLocaleString()}</p>
                          <p className='text-muted-foreground text-xs'>{renderCost(cost)}</p>
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              );
            })()}

          {/* Request and Response Tabs */}
          <Card className='border-0 shadow-sm'>
            <CardContent className='p-0'>
              <Tabs defaultValue='request' className='w-full'>
                <div className='bg-muted/20 border-b px-6 pt-6'>
                  <TabsList className='bg-background grid w-full grid-cols-3'>
                    <TabsTrigger value='request' className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'>
                      {t('requests.detail.tabs.request')}
                    </TabsTrigger>
                    <TabsTrigger value='response' className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'>
                      {t('requests.detail.tabs.response')}
                    </TabsTrigger>
                    <TabsTrigger value='executions' className='data-[state=active]:bg-primary data-[state=active]:text-primary-foreground'>
                      {t('requests.detail.tabs.executions')}
                    </TabsTrigger>
                  </TabsList>
                </div>

                <TabsContent value='request' className='space-y-6 p-6'>
                  {request.requestHeaders && (
                    <div className='space-y-4'>
                      <div className='flex items-center justify-between'>
                        <h4 className='flex items-center gap-2 text-base font-semibold'>
                          <FileText className='text-primary h-4 w-4' />
                          {t('requests.columns.requestHeaders')}
                        </h4>
                        <div className='flex gap-2'>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => copyToClipboard(formatJson(request.requestHeaders))}
                            className='hover:bg-primary hover:text-primary-foreground'
                          >
                            <Copy className='mr-2 h-4 w-4' />
                            {t('requests.dialogs.jsonViewer.copy')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => downloadFile(formatJson(request.requestHeaders), `request-headers-${request.id}.json`)}
                            className='hover:bg-primary hover:text-primary-foreground'
                          >
                            <Download className='mr-2 h-4 w-4' />
                            {t('requests.dialogs.jsonViewer.download')}
                          </Button>
                        </div>
                      </div>
                      <div className='bg-muted/20 h-[300px] w-full overflow-auto rounded-lg border p-4'>
                        <JsonViewer data={request.requestHeaders} rootName='' defaultExpanded={true} className='text-sm' />
                      </div>
                    </div>
                  )}
                  <div className='space-y-4'>
                    <div className='flex items-center justify-between'>
                      <h4 className='flex items-center gap-2 text-base font-semibold'>
                        <FileText className='text-primary h-4 w-4' />
                        {t('requests.columns.requestBody')}
                      </h4>
                      <div className='flex gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => copyToClipboard(formatJson(request.requestBody))}
                          className='hover:bg-primary hover:text-primary-foreground'
                        >
                          <Copy className='mr-2 h-4 w-4' />
                          {t('requests.dialogs.jsonViewer.copy')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => downloadFile(formatJson(request.requestBody), `request-body-${request.id}.json`)}
                          className='hover:bg-primary hover:text-primary-foreground'
                        >
                          <Download className='mr-2 h-4 w-4' />
                          {t('requests.dialogs.jsonViewer.download')}
                        </Button>
                      </div>
                    </div>
                    <div className='bg-muted/20 h-[500px] w-full overflow-auto rounded-lg border p-4'>
                      <JsonViewer data={request.requestBody} rootName='' defaultExpanded={true} className='text-sm' />
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
                      <div className='flex gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={showResponseChunksModal}
                          disabled={!request?.responseChunks || request.responseChunks.length === 0}
                          className='hover:bg-primary hover:text-primary-foreground disabled:opacity-50'
                        >
                          <Layers className='mr-2 h-4 w-4' />
                          {t('requests.columns.responseChunks')}
                        </Button>
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
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => downloadFile(formatJson(request.responseBody), `response-body-${request.id}.json`)}
                          disabled={!request.responseBody}
                          className='hover:bg-primary hover:text-primary-foreground disabled:opacity-50'
                        >
                          <Download className='mr-2 h-4 w-4' />
                          {t('requests.dialogs.jsonViewer.download')}
                        </Button>
                      </div>
                    </div>
                    {request.responseBody ? (
                      <div className='bg-muted/20 h-[500px] w-full overflow-auto rounded-lg border p-4'>
                        <JsonViewer data={request.responseBody} rootName='' defaultExpanded={true} className='text-sm' />
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
                        const execution = edge.node;
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
                              <div className='grid grid-cols-1 gap-4 sm:grid-cols-5'>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='flex items-center gap-2 text-sm font-medium'>
                                    <Database className='text-primary h-4 w-4' />
                                    {t('requests.columns.channel')}
                                  </span>
                                  <p className='text-muted-foreground font-mono text-sm'>
                                    {execution.channel?.name || t('requests.columns.unknown')}
                                  </p>
                                </div>
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
                                    {execution.status === 'completed' || execution.status === 'failed'
                                      ? execution.updatedAt
                                        ? format(new Date(execution.updatedAt), 'yyyy-MM-dd HH:mm:ss', { locale })
                                        : t('requests.columns.unknown')
                                      : '-'}
                                  </p>
                                </div>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='flex items-center gap-2 text-sm font-medium'>
                                    <Clock className='text-primary h-4 w-4' />
                                    {t('requests.columns.latency')}
                                  </span>
                                  <p className='text-muted-foreground font-mono text-sm'>
                                    {execution.status === 'completed' || execution.status === 'failed'
                                      ? formatLatency(calculateLatency(execution.createdAt, execution.updatedAt))
                                      : '-'}
                                  </p>
                                </div>
                                <div className='bg-background space-y-2 rounded-lg border p-3'>
                                  <span className='flex items-center gap-2 text-sm font-medium'>
                                    <Clock className='text-primary h-4 w-4' />
                                    {t('requests.columns.firstTokenLatency')}
                                  </span>
                                  <p className='text-muted-foreground font-mono text-sm'>
                                    {execution.status === 'completed' && execution.metricsFirstTokenLatencyMs != null
                                      ? formatLatency(execution.metricsFirstTokenLatencyMs)
                                      : '-'}
                                  </p>
                                </div>
                              </div>

                              {execution.errorMessage && (
                                <div className='bg-destructive/5 border-destructive/20 space-y-3 rounded-lg border p-4'>
                                  <span className='text-destructive flex items-center gap-2 text-sm font-semibold'>
                                    <FileText className='h-4 w-4' />
                                    {t('common.messages.errorMessage')}
                                  </span>
                                  <p className='text-destructive bg-destructive/10 rounded border p-3 text-sm'>{execution.errorMessage}</p>
                                </div>
                              )}

                              {execution.requestHeaders && (
                                <div className='space-y-3'>
                                  <div className='flex items-center justify-between'>
                                    <span className='flex items-center gap-2 text-sm font-semibold'>
                                      <FileText className='text-primary h-4 w-4' />
                                      {t('requests.columns.requestHeaders')}
                                    </span>
                                    <div className='flex gap-2'>
                                      <Button
                                        variant='outline'
                                        size='sm'
                                        onClick={() => copyToClipboard(formatJson(execution.requestHeaders))}
                                        className='hover:bg-primary hover:text-primary-foreground'
                                      >
                                        <Copy className='mr-2 h-4 w-4' />
                                        {t('requests.dialogs.jsonViewer.copy')}
                                      </Button>
                                      <Button
                                        variant='outline'
                                        size='sm'
                                        onClick={() => downloadFile(formatJson(execution.requestHeaders), `execution-${execution.id}-request-headers.json`)}
                                        className='hover:bg-primary hover:text-primary-foreground'
                                      >
                                        <Download className='mr-2 h-4 w-4' />
                                        {t('requests.dialogs.jsonViewer.download')}
                                      </Button>
                                    </div>
                                  </div>
                                  <div className='bg-background h-64 w-full overflow-auto rounded-lg border p-3'>
                                    <JsonViewer data={execution.requestHeaders} rootName='' defaultExpanded={false} className='text-xs' />
                                  </div>
                                </div>
                              )}

                              {execution.requestBody && (
                                <div className='space-y-3'>
                                  <div className='flex items-center justify-between'>
                                    <span className='flex items-center gap-2 text-sm font-semibold'>
                                      <FileText className='text-primary h-4 w-4' />
                                      {t('requests.columns.requestBody')}
                                    </span>
                                    <div className='flex gap-2'>
                                      <Button
                                        variant='outline'
                                        size='sm'
                                        onClick={() => copyToClipboard(formatJson(execution.requestBody))}
                                        className='hover:bg-primary hover:text-primary-foreground'
                                      >
                                        <Copy className='mr-2 h-4 w-4' />
                                        {t('requests.dialogs.jsonViewer.copy')}
                                      </Button>
                                      <Button
                                        variant='outline'
                                        size='sm'
                                        onClick={() => downloadFile(formatJson(execution.requestBody), `execution-${execution.id}-request-body.json`)}
                                        className='hover:bg-primary hover:text-primary-foreground'
                                      >
                                        <Download className='mr-2 h-4 w-4' />
                                        {t('requests.dialogs.jsonViewer.download')}
                                      </Button>
                                    </div>
                                  </div>
                                  <div className='bg-background h-64 w-full overflow-auto rounded-lg border p-3'>
                                    <JsonViewer data={execution.requestBody} rootName='' defaultExpanded={false} className='text-xs' />
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
                                    <div className='flex gap-2'>
                                      {execution.responseChunks && execution.responseChunks.length > 0 && (
                                        <Button
                                          variant='outline'
                                          size='sm'
                                          onClick={() => showExecutionChunksModal(execution.responseChunks)}
                                          className='hover:bg-primary hover:text-primary-foreground'
                                        >
                                          <Layers className='mr-2 h-4 w-4' />
                                          {t('requests.columns.responseChunks')}
                                        </Button>
                                      )}
                                      <Button
                                        variant='outline'
                                        size='sm'
                                        onClick={() => copyToClipboard(formatJson(execution.responseBody))}
                                        className='hover:bg-primary hover:text-primary-foreground'
                                      >
                                        <Copy className='mr-2 h-4 w-4' />
                                        {t('requests.dialogs.jsonViewer.copy')}
                                      </Button>
                                      <Button
                                        variant='outline'
                                        size='sm'
                                        onClick={() => downloadFile(formatJson(execution.responseBody), `execution-${execution.id}-response-body.json`)}
                                        className='hover:bg-primary hover:text-primary-foreground'
                                      >
                                        <Download className='mr-2 h-4 w-4' />
                                        {t('requests.dialogs.jsonViewer.download')}
                                      </Button>
                                    </div>
                                  </div>
                                  <div className='bg-background h-64 w-full overflow-auto rounded-lg border p-3'>
                                    <JsonViewer data={execution.responseBody} rootName='' defaultExpanded={false} className='text-xs' />
                                  </div>
                                </div>
                              )}
                            </CardContent>
                          </Card>
                        );
                      })}
                    </div>
                  ) : (
                    <div className='py-16 text-center'>
                      <div className='space-y-4'>
                        <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                        <p className='text-muted-foreground text-lg'>{t('requests.dialogs.requestDetail.noExecutions')}</p>
                      </div>
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </div>
      </Main>

      {/* Response Chunks Modal */}
      <ChunksDialog
        open={showResponseChunks}
        onOpenChange={setShowResponseChunks}
        chunks={selectedResponseChunks}
        title={t('requests.dialogs.jsonViewer.responseChunks')}
      />

      {/* Execution Chunks Modal */}
      <ChunksDialog
        open={showExecutionChunks}
        onOpenChange={setShowExecutionChunks}
        chunks={selectedExecutionChunks}
        title={t('requests.dialogs.jsonViewer.responseChunks')}
      />
    </div>
  );
}
