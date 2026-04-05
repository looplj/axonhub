'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  FileText,
  ChevronsDownUp,
  ChevronsUpDown,
  Copy,
  Terminal,
} from 'lucide-react';
import { toast } from 'sonner';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { JsonViewer } from '@/components/json-tree-view';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import { useRequest, fetchAdjacentRequestPage } from '../data';
import { Request, RequestConnection } from '../data/schema';
import { CurlPreviewDialog } from './curl-preview-dialog';
import { getStatusColor } from './help';
import { generateRequestCurl } from '../utils/curl-generator';

interface RequestBodyDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** ID of the request that was clicked. */
  initialRequestId: string | null;
  /** Position of initialRequestId within initialRequests. */
  initialIndex: number;
  /** Current page's request list (DESC order). */
  initialRequests: Request[];
  pageInfo?: RequestConnection['pageInfo'];
  /** Optional server-side filter currently applied to the table. */
  queryWhere?: Record<string, any>;
}

export function RequestBodyDrawer({
  open,
  onOpenChange,
  initialRequestId,
  initialIndex,
  initialRequests,
  pageInfo: initialPageInfo,
  queryWhere,
}: RequestBodyDrawerProps) {
  const { t } = useTranslation();
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 });
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();

  // ── internal navigation state ──────────────────────────────────────────────
  // The drawer manages its own growing list so it can cross page boundaries.
  const [allRequests, setAllRequests] = useState<Request[]>(initialRequests);
  const [navPageInfo, setNavPageInfo] = useState(initialPageInfo);
  const [currentIndex, setCurrentIndex] = useState(initialIndex);
  const [isLoadingMore, setIsLoadingMore] = useState(false);

  // Reset when the drawer is (re)opened for a different request.
  const prevOpenRef = useRef(false);
  useEffect(() => {
    const justOpened = open && !prevOpenRef.current;
    prevOpenRef.current = open;
    if (justOpened) {
      setAllRequests(initialRequests);
      setNavPageInfo(initialPageInfo);
      setCurrentIndex(initialIndex);
    }
  }, [open, initialRequests, initialPageInfo, initialIndex]);

  const currentRequestId = allRequests[currentIndex]?.id ?? initialRequestId;

  // ── toggle for expanding/collapsing all string values ────────────────────
  const [globalExpanded, setGlobalExpanded] = useState(false);

  // ── fetch detail for current request ──────────────────────────────────────
  const { data: request, isLoading, isFetching } = useRequest(currentRequestId ?? '');

  // Keep previous request data visible while loading the next one.
  const displayedRequestRef = useRef<Request | null>(null);
  if (request) displayedRequestRef.current = request;
  const displayedRequest = displayedRequestRef.current;

  // ── active tab ─────────────────────────────────────────────────────────────
  const [activeTab, setActiveTab] = useState('request');

  // ── copy / curl ───────────────────────────────────────────────────────────
  const [showCurlPreview, setShowCurlPreview] = useState(false);
  const [curlCommand, setCurlCommand] = useState('');

  const copyBody = useCallback(
    (data: any) => {
      try {
        navigator.clipboard.writeText(JSON.stringify(data, null, 2));
      } catch {
        navigator.clipboard.writeText(String(data));
      }
      toast.success(t('requests.actions.copy'));
    },
    [t]
  );

  const handleCurlPreview = useCallback(() => {
    if (!displayedRequest) return;
    const curl = generateRequestCurl(displayedRequest.requestHeaders, displayedRequest.requestBody, displayedRequest.format as any);
    setCurlCommand(curl);
    setShowCurlPreview(true);
  }, [displayedRequest]);

  // List-level data (always available, no loading flash).
  const listRequest = allRequests[currentIndex];

  // ── navigation ─────────────────────────────────────────────────────────────
  // The list is DESC (newest first).
  // → right arrow = "next" = newer = smaller index.
  // ← left  arrow = "prev" = older = larger index.
  const canGoPrev = currentIndex < allRequests.length - 1 || !!navPageInfo?.hasNextPage;
  const canGoNext = currentIndex > 0 || !!navPageInfo?.hasPreviousPage;

  const handlePrev = useCallback(async () => {
    if (currentIndex < allRequests.length - 1) {
      setCurrentIndex((i) => i + 1);
      return;
    }
    // Need to load the next (older) page.
    if (!navPageInfo?.hasNextPage || !navPageInfo.endCursor || isLoadingMore) return;
    setIsLoadingMore(true);
    try {
      const result = await fetchAdjacentRequestPage({
        cursor: navPageInfo.endCursor,
        direction: 'older',
        pageSize: initialRequests.length || 20,
        where: queryWhere,
        permissions,
        projectId: selectedProjectId,
      });
      setAllRequests((prev) => {
        const merged = [...prev, ...result.requests];
        setCurrentIndex(prev.length); // first item of the new batch
        return merged;
      });
      setNavPageInfo((p) =>
        p
          ? { ...p, hasNextPage: result.pageInfo.hasNextPage, endCursor: result.pageInfo.endCursor }
          : result.pageInfo
      );
    } finally {
      setIsLoadingMore(false);
    }
  }, [currentIndex, allRequests.length, navPageInfo, isLoadingMore, queryWhere, permissions, selectedProjectId, initialRequests.length]);

  const handleNext = useCallback(async () => {
    if (currentIndex > 0) {
      setCurrentIndex((i) => i - 1);
      return;
    }
    // Need to load the previous (newer) page.
    if (!navPageInfo?.hasPreviousPage || !navPageInfo.startCursor || isLoadingMore) return;
    setIsLoadingMore(true);
    try {
      const result = await fetchAdjacentRequestPage({
        cursor: navPageInfo.startCursor,
        direction: 'newer',
        pageSize: initialRequests.length || 20,
        where: queryWhere,
        permissions,
        projectId: selectedProjectId,
      });
      // Prepend newer items; adjust index for shift.
      setAllRequests((prev) => {
        const merged = [...result.requests, ...prev];
        // Navigate to the newest item in the just-fetched batch.
        setCurrentIndex(result.requests.length - 1);
        return merged;
      });
      setNavPageInfo((p) =>
        p
          ? { ...p, hasPreviousPage: result.pageInfo.hasPreviousPage, startCursor: result.pageInfo.startCursor }
          : result.pageInfo
      );
    } finally {
      setIsLoadingMore(false);
    }
  }, [currentIndex, navPageInfo, isLoadingMore, queryWhere, permissions, selectedProjectId, initialRequests.length]);

  const handleViewDetail = useCallback(() => {
    if (currentRequestId) {
      navigateWithSearch({
        to: '/project/requests/$requestId',
        params: { requestId: currentRequestId },
      });
      onOpenChange(false);
    }
  }, [currentRequestId, navigateWithSearch, onOpenChange]);

  // ── render ─────────────────────────────────────────────────────────────────
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side='right'
        className='flex w-[50vw] min-w-[500px] max-w-[800px] flex-col gap-0 p-0 sm:max-w-[800px]'
      >
        {/* Header */}
        <SheetHeader className='flex-shrink-0 border-b px-6 py-4'>
          <div className='flex items-center justify-between pr-6'>
            <SheetTitle className='flex items-center gap-2 text-base'>
              <FileText className='h-4 w-4' />
              {listRequest ? (
                <>
                  <span className='font-mono'>#{extractNumberID(listRequest.id)}</span>
                  <Badge className={getStatusColor(listRequest.status)} variant='secondary'>
                    {t(`requests.status.${listRequest.status}`)}
                  </Badge>
                </>
              ) : isLoading ? (
                <Skeleton className='h-4 w-16' />
              ) : null}
            </SheetTitle>

            <div className='flex items-center gap-1.5'>
              <Button
                variant='outline'
                size='icon'
                className='h-7 w-7'
                onClick={handlePrev}
                disabled={!canGoPrev || isLoadingMore}
                title={t('requests.drawer.previous')}
              >
                <ChevronLeft className='h-4 w-4' />
              </Button>
              <Button
                variant='outline'
                size='icon'
                className='h-7 w-7'
                onClick={handleNext}
                disabled={!canGoNext || isLoadingMore}
                title={t('requests.drawer.next')}
              >
                <ChevronRight className='h-4 w-4' />
              </Button>
              <Button
                variant='outline'
                size='sm'
                onClick={handleViewDetail}
                className='ml-1 h-7 text-xs'
              >
                <ExternalLink className='mr-1 h-3.5 w-3.5' />
                {t('requests.drawer.viewDetail')}
              </Button>
            </div>
          </div>
        </SheetHeader>

        {/* Body */}
        <div className='flex min-h-0 flex-1 flex-col'>
          {displayedRequest ? (
            <div className='relative flex min-h-0 flex-1 flex-col'>
              {isFetching && (
                <div className='absolute inset-x-0 top-0 z-10 h-0.5 animate-pulse bg-primary/40' />
              )}
              <Tabs value={activeTab} onValueChange={setActiveTab} className='flex h-full flex-col'>
                {/* Tab bar + action buttons */}
                <div className='mx-6 mt-4 flex flex-shrink-0 items-center gap-2'>
                  <TabsList className='grid flex-1 grid-cols-2'>
                    <TabsTrigger value='request'>{t('requests.detail.tabs.request')}</TabsTrigger>
                    <TabsTrigger value='response'>{t('requests.detail.tabs.response')}</TabsTrigger>
                  </TabsList>
                  <Button
                    variant='outline'
                    size='icon'
                    className='h-9 w-9 flex-shrink-0'
                    onClick={() => setGlobalExpanded((v) => !v)}
                    title={globalExpanded ? t('requests.drawer.collapseAll') : t('requests.drawer.expandAll')}
                  >
                    {globalExpanded ? (
                      <ChevronsDownUp className='h-4 w-4' />
                    ) : (
                      <ChevronsUpDown className='h-4 w-4' />
                    )}
                  </Button>
                  <Button
                    variant='outline'
                    size='icon'
                    className='h-9 w-9 flex-shrink-0'
                    onClick={() =>
                      copyBody(activeTab === 'request' ? displayedRequest.requestBody : displayedRequest.responseBody)
                    }
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

                <TabsContent value='request' className='m-0 min-h-0 flex-1 px-6 pb-6 pt-4'>
                  <ScrollArea className='bg-muted/20 h-full w-full rounded-lg border p-4'>
                    {displayedRequest.requestBody ? (
                      <JsonViewer
                        key={`req-${currentRequestId}`}
                        data={displayedRequest.requestBody}
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
                </TabsContent>

                <TabsContent value='response' className='m-0 min-h-0 flex-1 px-6 pb-6 pt-4'>
                  <ScrollArea className='bg-muted/20 h-full w-full rounded-lg border p-4'>
                    {displayedRequest.responseBody ? (
                      <JsonViewer
                        key={`res-${currentRequestId}`}
                        data={displayedRequest.responseBody}
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
                </TabsContent>
              </Tabs>
            </div>
          ) : isLoading ? (
            <div className='space-y-4 p-6'>
              <Skeleton className='h-8 w-full' />
              <Skeleton className='h-64 w-full' />
              <Skeleton className='h-32 w-full' />
            </div>
          ) : null}
        </div>
      </SheetContent>
      <CurlPreviewDialog open={showCurlPreview} onOpenChange={setShowCurlPreview} curlCommand={curlCommand} />
    </Sheet>
  );
}
