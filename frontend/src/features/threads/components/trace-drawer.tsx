import { useMemo, useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, FileSearch, List, GitBranch, Waypoints, Maximize2, Minimize2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { SpanSection } from '@/features/traces/components/span-section';
import { TraceFlatTimeline } from '@/features/traces/components/trace-flat-timeline';
import { TraceFlowTimeline } from '@/features/traces/components/trace-flow-timeline';
import { TraceTreeTimeline } from '@/features/traces/components/trace-tree-view';
import { useTraceWithSegments } from '@/features/traces/data';
import { Segment, Span, parseRawRootSegment } from '@/features/traces/data/schema';

interface TraceDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  traceId: string | null;
}

export function TraceDrawer({ open, onOpenChange, traceId }: TraceDrawerProps) {
  const { t } = useTranslation();
  const [selectedTrace, setSelectedTrace] = useState<Segment | null>(null);
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null);
  const [selectedSpanType, setSelectedSpanType] = useState<'request' | 'response' | null>(null);
  const [viewMode, setViewMode] = useState<'flat' | 'flow' | 'tree'>('flow');
  const [isFullscreen, setIsFullscreen] = useState(false);

  const { data: trace, isLoading } = useTraceWithSegments(traceId || '');

  // Parse rawRootSegment JSON once per trace
  const effectiveRootSegment = useMemo(() => {
    if (!trace?.rawRootSegment) return null;
    return parseRawRootSegment(trace.rawRootSegment);
  }, [trace]);

  // Auto-select first span when trace loads
  useEffect(() => {
    if (effectiveRootSegment && !selectedSpan) {
      const firstSpan = effectiveRootSegment.requestSpans?.[0] || effectiveRootSegment.responseSpans?.[0];
      if (firstSpan) {
        const spanType = effectiveRootSegment.requestSpans?.[0] ? 'request' : 'response';
        setSelectedTrace(effectiveRootSegment);
        setSelectedSpan(firstSpan);
        setSelectedSpanType(spanType);
      }
    }
  }, [effectiveRootSegment, selectedSpan]);

  const handleSpanSelect = (parentTrace: Segment, span: Span, type: 'request' | 'response') => {
    setSelectedTrace(parentTrace);
    setSelectedSpan(span);
    setSelectedSpanType(type);
  };

  // Reset state when drawer closes
  useEffect(() => {
    if (!open) {
      setSelectedTrace(null);
      setSelectedSpan(null);
      setSelectedSpanType(null);
      setViewMode('flow');
      setIsFullscreen(false);
    }
  }, [open]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className={cn('w-full p-0 transition-all duration-300', isFullscreen ? 'sm:max-w-none' : 'sm:max-w-[900px] lg:max-w-[1100px]')}>
        <SheetHeader className={cn(
          'border-b border-border/50 bg-gradient-to-r from-background via-background to-muted/20 px-6 py-4',
          isFullscreen && 'fixed top-0 left-0 right-0 z-50'
        )}>
          <div className='flex items-center justify-between'>
            <SheetTitle className='flex items-center gap-2 text-lg font-semibold'>
              <span className='h-5 w-1 rounded-full bg-gradient-to-b from-primary/60 via-primary to-primary/60' />
              {t('traces.detail.title')}
            </SheetTitle>
            
            {/* View Mode Switcher */}
            <div className='flex items-center'>
              <div className='bg-muted inline-flex items-center rounded-md p-0.5'>
                <Button
                  variant='ghost'
                  size='sm'
                  className={cn('h-7 gap-1.5 rounded-sm px-2.5 text-xs', viewMode === 'flat' && 'bg-background shadow-sm')}
                  onClick={() => setViewMode('flat')}
                >
                  <List className='h-3.5 w-3.5' />
                  {t('traces.detail.viewMode.flat')}
                </Button>
                <Button
                  variant='ghost'
                  size='sm'
                  className={cn('h-7 gap-1.5 rounded-sm px-2.5 text-xs', viewMode === 'flow' && 'bg-background shadow-sm')}
                  onClick={() => setViewMode('flow')}
                >
                  <GitBranch className='h-3.5 w-3.5' />
                  {t('traces.detail.viewMode.flow')}
                </Button>
                <Button
                  variant='ghost'
                  size='sm'
                  className={cn('h-7 gap-1.5 rounded-sm px-2.5 text-xs', viewMode === 'tree' && 'bg-background shadow-sm')}
                  onClick={() => setViewMode('tree')}
                >
                  <Waypoints className='h-3.5 w-3.5' />
                  {t('traces.detail.viewMode.tree')}
                </Button>
              </div>

              {/* Fullscreen Toggle */}
              <Button
                variant='ghost'
                size='sm'
                className='ml-3 h-8 w-8 p-0'
                onClick={() => setIsFullscreen(!isFullscreen)}
                title={isFullscreen ? t('common.exitFullscreen') : t('common.fullscreen')}
              >
                {isFullscreen ? <Minimize2 className='h-4 w-4' /> : <Maximize2 className='h-4 w-4' />}
              </Button>

              {/* Close Button (only in fullscreen) */}
              {isFullscreen && (
                <Button
                  variant='ghost'
                  size='sm'
                  className='ml-1.5 h-8 w-8 p-0'
                  onClick={() => onOpenChange(false)}
                >
                  <X className='h-4 w-4' />
                </Button>
              )}

              <Separator orientation='vertical' className='ml-3 h-6' />
            </div>
          </div>
        </SheetHeader>

        {isLoading ? (
          <div className='flex h-[calc(100vh-80px)] items-center justify-center bg-gradient-to-b from-background to-muted/20'>
            <div className='space-y-4 text-center'>
              <div className='relative mx-auto h-12 w-12'>
                <div className='absolute inset-0 rounded-full border-2 border-primary/10' />
                <div className='absolute inset-0 rounded-full border-2 border-transparent border-t-primary animate-spin' />
                <Loader2 className='absolute inset-0 m-auto h-6 w-6 text-primary/60 animate-spin' />
              </div>
              <p className='text-muted-foreground text-sm font-medium'>{t('common.loading')}</p>
            </div>
          </div>
        ) : effectiveRootSegment ? (
          <div className={cn(
            'flex bg-gradient-to-br from-background via-background to-muted/10',
            isFullscreen ? 'fixed inset-0 z-40 pt-16' : 'h-[calc(100vh-80px)]'
          )}>
            {/* Left: Timeline */}
            <div className={cn(
              'flex-1 min-w-0 overflow-auto',
              isFullscreen ? 'p-4' : 'p-6'
            )}>
              {viewMode === 'flat' ? (
                <TraceFlatTimeline
                  trace={effectiveRootSegment}
                  onSelectSpan={(selectedTrace, span, type) => handleSpanSelect(selectedTrace, span, type)}
                  selectedSpanId={selectedSpan?.id}
                />
              ) : viewMode === 'flow' ? (
                <TraceFlowTimeline
                  trace={effectiveRootSegment}
                  onSelectSpan={(selectedTrace, span, type) => handleSpanSelect(selectedTrace, span, type)}
                  selectedSpanId={selectedSpan?.id}
                  isFullscreen={isFullscreen}
                />
              ) : (
                <TraceTreeTimeline
                  trace={effectiveRootSegment}
                  onSelectSpan={(selectedTrace, span, type) => handleSpanSelect(selectedTrace, span, type)}
                  selectedSpanId={selectedSpan?.id}
                />
              )}
            </div>

            {/* Right: Span Detail */}
            <div className={cn(
              'border-border/50 bg-card/50 shrink-0 overflow-y-auto border-l backdrop-blur-sm transition-all duration-300',
              isFullscreen ? 'w-[420px]' : 'w-[380px] lg:w-[420px]'
            )}>
              <SpanSection selectedTrace={selectedTrace} selectedSpan={selectedSpan} selectedSpanType={selectedSpanType} />
            </div>
          </div>
        ) : (
          <div className={cn(
            'flex items-center justify-center bg-gradient-to-b from-background to-muted/20',
            isFullscreen ? 'fixed inset-0 z-40 pt-16' : 'h-[calc(100vh-80px)]'
          )}>
            <div className='space-y-4 text-center'>
              <div className='relative mx-auto h-16 w-16'>
                <div className='absolute inset-0 rounded-full bg-muted/50' />
                <div className='absolute inset-0 flex items-center justify-center'>
                  <FileSearch className='h-7 w-7 text-muted-foreground/60' />
                </div>
              </div>
              <div className='space-y-1'>
                <p className='text-foreground font-medium'>{t('traces.detail.noTraceData')}</p>
                <p className='text-muted-foreground text-sm'>{t('traces.detail.checkTraceIdHint')}</p>
              </div>
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
