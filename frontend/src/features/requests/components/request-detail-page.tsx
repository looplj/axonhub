import { useState, useCallback, useEffect, useMemo, useRef } from 'react';
import { format } from 'date-fns';
import { useParams, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useStoragePolicy } from '@/features/system/data/system';
import { useSelectedProjectId } from '@/stores/projectStore';
import { getTokenFromStorage } from '@/stores/authStore';
import { type Request, useRequest } from '../data';
import { RequestDetailContent } from './request-detail-content';

type PreviewFallbackResponse = {
  mode?: string;
  responseChunks?: any[];
};

type PreviewEvent = {
  event: string;
  data: string;
};

function parsePreviewEvent(rawEvent: string): PreviewEvent | null {
  const normalizedEvent = rawEvent.replace(/\r\n/g, '\n').trim();
  if (!normalizedEvent) return null;

  let event = 'message';
  const dataLines: string[] = [];

  normalizedEvent.split('\n').forEach((line) => {
    if (!line || line.startsWith(':')) {
      return;
    }

    if (line.startsWith('event:')) {
      event = line.slice(6).trim();
      return;
    }

    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart());
    }
  });

  return {
    event,
    data: dataLines.join('\n'),
  };
}

function parsePreviewChunk(payload: string): any {
  try {
    return JSON.parse(payload);
  } catch {
    return payload;
  }
}

async function readPreviewStream(
  response: Response,
  handlers: {
    onEvent: (event: PreviewEvent) => Promise<void> | void;
  }
) {
  const reader = response.body?.getReader();
  if (!reader) {
    throw new Error('Preview stream body is not readable');
  }

  const decoder = new TextDecoder();
  let buffer = '';

  const flushBuffer = async (final = false) => {
    const normalizedBuffer = buffer.replace(/\r\n/g, '\n');
    const separator = '\n\n';
    let separatorIndex = normalizedBuffer.indexOf(separator);

    while (separatorIndex !== -1) {
      const rawEvent = normalizedBuffer.slice(0, separatorIndex);
      buffer = normalizedBuffer.slice(separatorIndex + separator.length);

      const event = parsePreviewEvent(rawEvent);
      if (event) {
        await handlers.onEvent(event);
      }

      return flushBuffer(final);
    }

    if (final && normalizedBuffer.trim()) {
      const event = parsePreviewEvent(normalizedBuffer);
      if (event) {
        await handlers.onEvent(event);
      }
      buffer = '';
    } else {
      buffer = normalizedBuffer;
    }
  };

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      buffer += decoder.decode();
      await flushBuffer(true);
      return;
    }

    buffer += decoder.decode(value, { stream: true });
    await flushBuffer();
  }
}

export default function RequestDetailPage() {
  const { t } = useTranslation();
  const { requestId } = useParams({ from: '/_authenticated/project/requests/$requestId' });
  const navigate = useNavigate();
  const { getSearchParams } = usePaginationSearch({ defaultPageSize: 20 });
  const selectedProjectId = useSelectedProjectId();
  const { data: storagePolicy } = useStoragePolicy();
  const isLivePreviewEnabled = storagePolicy?.livePreview ?? false;
  const [previewRequest, setPreviewRequest] = useState<Request | null>(null);
  const [isPreviewStreaming, setIsPreviewStreaming] = useState(false);
  const [previewFallbackActive, setPreviewFallbackActive] = useState(false);
  const previewCompletedRef = useRef(false);
  const previewChunkCountRef = useRef(0);

  const {
    data: requestData,
    refetch: refetchRequest,
  } = useRequest(requestId, { projectId: selectedProjectId, disableAutoRefresh: isPreviewStreaming });

  const request = previewRequest ?? requestData;

  useEffect(() => {
    if (!requestData) {
      setPreviewRequest(null);
      setPreviewFallbackActive(false);
      return;
    }

    if (requestData.status !== 'processing' || !requestData.stream) {
      setPreviewRequest(null);
      setIsPreviewStreaming(false);
      setPreviewFallbackActive(false);
      previewCompletedRef.current = false;
      previewChunkCountRef.current = 0;
    }
  }, [requestData]);

  useEffect(() => {
    if (!isLivePreviewEnabled) {
      setPreviewRequest(null);
      setIsPreviewStreaming(false);
      setPreviewFallbackActive(false);
      previewCompletedRef.current = false;
      previewChunkCountRef.current = 0;
      return;
    }

    if (!requestData || requestData.status !== 'processing' || !requestData.stream) {
      return;
    }

    if (previewFallbackActive) {
      return;
    }

    if (!selectedProjectId) {
      return;
    }

    const token = getTokenFromStorage();
    if (!token) {
      return;
    }

    const requestIdNumber = extractNumberID(requestData.id);
    if (!requestIdNumber) {
      return;
    }

    const controller = new AbortController();
    let isDisposed = false;
    let reconnectTimer: ReturnType<typeof window.setTimeout> | null = null;
    let reconnectAttempt = 0;

    previewCompletedRef.current = false;
    setIsPreviewStreaming(true);
    setPreviewFallbackActive(false);
    setPreviewRequest({
      ...requestData,
      responseChunks: [],
    });
    previewChunkCountRef.current = 0;

    const clearReconnectTimer = () => {
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    };

    const scheduleReconnect = () => {
      if (isDisposed || controller.signal.aborted) {
        return;
      }
      if (requestData.status !== 'processing' || !requestData.stream || previewCompletedRef.current) {
        return;
      }
      if (reconnectTimer !== null) {
        return;
      }

      reconnectAttempt += 1;
      const delay = Math.min(5000, 500 * reconnectAttempt);
      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = null;
        if (!isDisposed && !controller.signal.aborted) {
          setIsPreviewStreaming(true);
          void connectPreview();
        }
      }, delay);
    };

    async function connectPreview() {
      try {
        const response = await fetch(`/admin/requests/${encodeURIComponent(requestIdNumber)}/preview`, {
          headers: {
            Authorization: `Bearer ${token}`,
            'X-Project-ID': selectedProjectId,
          },
          signal: controller.signal,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const contentType = response.headers.get('content-type') || '';
        if (!contentType.includes('text/event-stream')) {
          const fallbackResponse = (await response.json()) as PreviewFallbackResponse;
          if (!isDisposed && fallbackResponse.mode === 'static-fetch') {
            setPreviewRequest((currentRequest) => currentRequest
              ? {
                  ...currentRequest,
                  responseChunks: fallbackResponse.responseChunks ?? currentRequest.responseChunks,
                }
              : currentRequest);
            setIsPreviewStreaming(false);
            setPreviewFallbackActive(true);
          } else if (!isDisposed) {
            setPreviewRequest(null);
            setIsPreviewStreaming(false);
            setPreviewFallbackActive(true);
          }
          return;
        }

        let replayChunksToSkip = previewChunkCountRef.current;

        await readPreviewStream(response, {
          onEvent: async ({ event, data }) => {
            if (isDisposed) {
              return;
            }

            if (event === 'preview.replay' || event === 'preview.chunk') {
              if (event === 'preview.replay' && replayChunksToSkip > 0) {
                replayChunksToSkip -= 1;
                return;
              }

              const nextChunk = parsePreviewChunk(data);
              previewChunkCountRef.current += 1;
              setPreviewRequest((currentRequest) => currentRequest
                ? {
                  ...currentRequest,
                  responseChunks: [...(currentRequest.responseChunks ?? []), nextChunk],
                }
                : currentRequest);
              return;
            }

            if (event === 'preview.completed') {
              setIsPreviewStreaming(false);
              setPreviewFallbackActive(false);

              if (!previewCompletedRef.current) {
                previewCompletedRef.current = true;
                await refetchRequest();
              }
            }
          },
        });

        if (!isDisposed && !controller.signal.aborted) {
          if (previewCompletedRef.current) {
            setIsPreviewStreaming(false);
            clearReconnectTimer();
            return;
          }

          scheduleReconnect();
        }
      } catch (error) {
        if (controller.signal.aborted || isDisposed) {
          return;
        }

        if (requestData.status === 'processing' && requestData.stream) {
          setIsPreviewStreaming(false);
          scheduleReconnect();
        } else {
          setPreviewRequest(null);
          setIsPreviewStreaming(false);
          setPreviewFallbackActive(true);
        }
      }
    }

    void connectPreview();

    return () => {
      isDisposed = true;
      setIsPreviewStreaming(false);
      clearReconnectTimer();
      controller.abort();
    };
  }, [isLivePreviewEnabled, previewFallbackActive, requestData, refetchRequest, selectedProjectId]);

  const handleBack = () => {
    navigate({
      to: '/project/requests',
      search: getSearchParams(),
    });
  };

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
                {t('requests.detail.title')} #{request ? extractNumberID(request.id) || request.id : extractNumberID(requestId) || requestId}
              </h1>
              {request && (
                <div className='mt-1 flex items-center gap-2'>
                  <p className='text-muted-foreground text-sm'>{request.modelID || t('requests.columns.unknown')}</p>
                  <span className='text-muted-foreground text-xs'>•</span>
                  <p className='text-muted-foreground text-xs'>{format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss')}</p>
                </div>
              )}
            </div>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-auto'>
        <div className='container mx-auto max-w-7xl p-6'>
          <RequestDetailContent
            requestId={requestId}
            projectId={selectedProjectId}
            previewRequest={previewRequest}
            isPreviewStreaming={isPreviewStreaming}
          />
        </div>
      </Main>
    </div>
  );
}
