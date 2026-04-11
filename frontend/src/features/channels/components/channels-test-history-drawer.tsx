'use client';

import { useMemo, useState, useEffect } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { format } from 'date-fns';
import {
  ChevronRight,
  ExternalLink,
  FileText,
  History,
  ChevronsDownUp,
  ChevronsUpDown,
  Copy,
  Terminal,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberID } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { JsonViewer } from '@/components/json-tree-view';
import { useRequests, useRequest } from '@/features/requests/data';
import { CurlPreviewDialog } from '@/features/requests/components/curl-preview-dialog';
import { generateRequestCurl } from '@/features/requests/utils/curl-generator';
import { getStatusColor } from '@/features/requests/components/help';
import { Channel } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  channel: Channel;
}

export function ChannelsTestHistoryDrawer({ open, onOpenChange, channel }: Props) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [selectedRequestId, setSelectedRequestId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState('request');
  const [globalExpanded, setGlobalExpanded] = useState(false);
  const [showCurlPreview, setShowCurlPreview] = useState(false);
  const [curlCommand, setCurlCommand] = useState('');

  const { data, isLoading } = useRequests(
    {
      first: 50,
      where: {
        channelID: channel.id,
        sourceIn: ['test'],
      },
      orderBy: {
        field: 'CREATED_AT',
        direction: 'DESC',
      },
    },
    { scopeToSelectedProject: false, projectId: null, enabled: open }
  );

  const requests = useMemo(() => data?.edges?.map((edge) => edge.node) || [], [data]);

  useEffect(() => {
    if (!open) {
      setSelectedRequestId(null);
      return;
    }

    if (!selectedRequestId && requests.length > 0) {
      setSelectedRequestId(requests[0].id);
    }
  }, [open, selectedRequestId, requests]);

  const { data: request, isLoading: isRequestLoading } = useRequest(selectedRequestId ?? '', {
    projectId: null,
    enabled: open && !!selectedRequestId,
  });

  const copyBody = (data: any) => {
    try {
      navigator.clipboard.writeText(JSON.stringify(data, null, 2));
    } catch {
      navigator.clipboard.writeText(String(data));
    }
    toast.success(t('requests.actions.copy'));
  };

  const handleCurlPreview = () => {
    if (!request) return;
    const curl = generateRequestCurl(request.requestHeaders, request.requestBody, request.format as any);
    setCurlCommand(curl);
    setShowCurlPreview(true);
  };

  const handleViewDetail = () => {
    if (!selectedRequestId) return;
    onOpenChange(false);
    navigate({
      to: '/requests/$requestId',
      params: { requestId: selectedRequestId },
    });
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className='w-[80vw] min-w-[720px] max-w-[1200px] p-0 sm:max-w-[1200px]'>
        <div className='flex h-full'>
          <div className='flex w-[360px] flex-shrink-0 flex-col border-r'>
            <SheetHeader className='border-b px-6 py-4 text-left'>
              <SheetTitle className='flex items-center gap-2 text-base'>
                <History className='h-4 w-4' />
                {t('channels.actions.testHistory')}
              </SheetTitle>
              <p className='text-muted-foreground text-sm'>{channel.name}</p>
            </SheetHeader>

            <ScrollArea className='flex-1'>
              <div className='p-4'>
                {isLoading ? (
                  <div className='space-y-3'>
                    {Array.from({ length: 6 }).map((_, index) => (
                      <Skeleton key={index} className='h-20 w-full rounded-lg' />
                    ))}
                  </div>
                ) : requests.length === 0 ? (
                  <div className='flex h-40 items-center justify-center text-center'>
                    <div className='space-y-2'>
                      <FileText className='text-muted-foreground mx-auto h-10 w-10' />
                      <p className='text-muted-foreground text-sm'>{t('channels.dialogs.testHistory.empty')}</p>
                    </div>
                  </div>
                ) : (
                  <div className='space-y-3'>
                    {requests.map((item) => (
                      <button
                        key={item.id}
                        type='button'
                        onClick={() => setSelectedRequestId(item.id)}
                        className={`hover:bg-muted/60 w-full rounded-lg border p-3 text-left transition-colors ${selectedRequestId === item.id ? 'bg-muted border-primary' : 'bg-background'}`}
                      >
                        <div className='flex items-start justify-between gap-3'>
                          <div className='min-w-0 flex-1 space-y-1'>
                            <div className='flex items-center gap-2'>
                              <span className='font-mono text-sm font-medium'>#{extractNumberID(item.id)}</span>
                              <Badge variant='secondary' className={getStatusColor(item.status)}>
                                {t(`requests.status.${item.status}`)}
                              </Badge>
                            </div>
                            <p className='truncate text-sm'>{item.modelID || t('requests.columns.unknown')}</p>
                            <p className='text-muted-foreground text-xs'>{format(new Date(item.createdAt), 'yyyy-MM-dd HH:mm:ss')}</p>
                          </div>
                          <ChevronRight className='text-muted-foreground h-4 w-4 flex-shrink-0' />
                        </div>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </ScrollArea>
          </div>

          <div className='flex min-w-0 flex-1 flex-col'>
            <SheetHeader className='flex-shrink-0 border-b px-6 py-4 text-left'>
              <div className='flex items-center justify-between gap-3'>
                <SheetTitle className='flex items-center gap-2 text-base'>
                  <FileText className='h-4 w-4' />
                  {selectedRequestId ? (
                    <span className='font-mono'>#{extractNumberID(selectedRequestId)}</span>
                  ) : (
                    t('channels.dialogs.testHistory.placeholder')
                  )}
                </SheetTitle>
                {selectedRequestId && (
                  <Button variant='outline' size='sm' onClick={handleViewDetail} className='h-8 text-xs'>
                    <ExternalLink className='mr-1 h-3.5 w-3.5' />
                    {t('requests.drawer.viewDetail')}
                  </Button>
                )}
              </div>
            </SheetHeader>

            {!selectedRequestId ? (
              <div className='flex h-full items-center justify-center p-6'>
                <div className='space-y-2 text-center'>
                  <History className='text-muted-foreground mx-auto h-10 w-10' />
                  <p className='text-sm font-medium'>{t('channels.dialogs.testHistory.placeholder')}</p>
                  <p className='text-muted-foreground text-sm'>{t('channels.dialogs.testHistory.placeholderDescription')}</p>
                </div>
              </div>
            ) : isRequestLoading ? (
              <div className='space-y-4 p-6'>
                <Skeleton className='h-8 w-full' />
                <Skeleton className='h-64 w-full' />
                <Skeleton className='h-32 w-full' />
              </div>
            ) : request ? (
              <div className='flex min-h-0 flex-1 flex-col'>
                <div className='mx-6 mt-4 flex flex-shrink-0 items-center gap-2'>
                  <Tabs value={activeTab} onValueChange={setActiveTab} className='flex h-full flex-1 flex-col'>
                    <TabsList className='grid flex-1 grid-cols-2'>
                      <TabsTrigger value='request'>{t('requests.detail.tabs.request')}</TabsTrigger>
                      <TabsTrigger value='response'>{t('requests.detail.tabs.response')}</TabsTrigger>
                    </TabsList>
                  </Tabs>
                  <Button
                    variant='outline'
                    size='icon'
                    className='h-9 w-9 flex-shrink-0'
                    onClick={() => setGlobalExpanded((v) => !v)}
                    title={globalExpanded ? t('requests.drawer.collapseAll') : t('requests.drawer.expandAll')}
                  >
                    {globalExpanded ? <ChevronsDownUp className='h-4 w-4' /> : <ChevronsUpDown className='h-4 w-4' />}
                  </Button>
                  <Button
                    variant='outline'
                    size='icon'
                    className='h-9 w-9 flex-shrink-0'
                    onClick={() => copyBody(activeTab === 'request' ? request.requestBody : request.responseBody)}
                    title={t('requests.actions.copy')}
                  >
                    <Copy className='h-4 w-4' />
                  </Button>
                  {activeTab === 'request' && (
                    <Button
                      variant='outline'
                      size='icon'
                      className='h-9 w-9 flex-shrink-0'
                      onClick={handleCurlPreview}
                      title={t('requests.actions.copyCurl')}
                    >
                      <Terminal className='h-4 w-4' />
                    </Button>
                  )}
                </div>

                <div className='min-h-0 flex-1 px-6 pb-6 pt-4'>
                  {activeTab === 'request' ? (
                    <ScrollArea className='bg-muted/20 h-full w-full rounded-lg border p-4'>
                      {request.requestBody ? (
                        <JsonViewer
                          data={request.requestBody}
                          rootName=''
                          defaultExpanded={true}
                          expandDepth='all'
                          hideArrayIndices={true}
                          globalStringExpanded={globalExpanded}
                          className='text-sm'
                        />
                      ) : (
                        <div className='flex h-32 items-center justify-center'>
                          <p className='text-muted-foreground text-sm'>{t('requests.drawer.noRequestBody')}</p>
                        </div>
                      )}
                    </ScrollArea>
                  ) : (
                    <ScrollArea className='bg-muted/20 h-full w-full rounded-lg border p-4'>
                      {request.responseBody ? (
                        <JsonViewer
                          data={request.responseBody}
                          rootName=''
                          defaultExpanded={true}
                          expandDepth='all'
                          hideArrayIndices={true}
                          globalStringExpanded={globalExpanded}
                          className='text-sm'
                        />
                      ) : (
                        <div className='flex h-32 items-center justify-center'>
                          <p className='text-muted-foreground text-sm'>{t('requests.detail.noResponse')}</p>
                        </div>
                      )}
                    </ScrollArea>
                  )}
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </SheetContent>
      <CurlPreviewDialog open={showCurlPreview} onOpenChange={setShowCurlPreview} curlCommand={curlCommand} />
    </Sheet>
  );
}
