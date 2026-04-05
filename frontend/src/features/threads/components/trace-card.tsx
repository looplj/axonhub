import { format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight, User, Bot, Clock } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import type { Trace } from '@/features/traces/data/schema';

interface TraceCardProps {
  trace: Trace;
  onViewTrace: (traceId: string) => void;
  index?: number;
}

export function TraceCard({ trace, onViewTrace, index }: TraceCardProps) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const createdAtLabel = format(trace.createdAt, 'yyyy-MM-dd HH:mm:ss', { locale });

  return (
    <Card className='group relative overflow-hidden border border-border/50 bg-gradient-to-br from-card to-card/95 shadow-sm transition-all duration-300 hover:shadow-lg hover:border-border hover:-translate-y-0.5'>
      {/* Top accent line */}
      <div className='absolute left-0 right-0 top-0 h-0.5 bg-gradient-to-r from-primary/60 via-primary to-primary/60 opacity-0 transition-opacity duration-300 group-hover:opacity-100' />
      
      <CardContent className='p-5'>
        <div className='space-y-4'>
          {/* Header: Index + Trace ID + Created At */}
          <div className='flex items-center justify-between'>
            <div className='flex items-center gap-2.5'>
              {index !== undefined && (
                <Badge 
                  variant='secondary' 
                  className='h-6 min-w-6 justify-center rounded-md px-2 font-mono text-xs font-medium'
                >
                  #{index + 1}
                </Badge>
              )}
              <div className='flex items-center gap-1.5 rounded-md bg-muted/50 px-2 py-1'>
                <span className='font-mono text-xs text-muted-foreground'>
                  {trace.traceID}
                </span>
              </div>
            </div>
            <div className='flex items-center gap-1.5 text-xs text-muted-foreground/80'>
              <Clock className='h-3 w-3' />
              <span>{createdAtLabel}</span>
            </div>
          </div>

          {/* Chat Messages */}
          <div className='space-y-4 pt-1'>
            {/* User Query */}
            {trace.firstUserQuery && (
              <div className='flex items-start justify-end gap-2.5'>
                <div className='flex max-w-[85%] flex-col items-end gap-1'>
                  <div className='relative rounded-2xl rounded-tr-sm bg-gradient-to-br from-primary to-primary/90 px-4 py-2.5 text-primary-foreground shadow-sm'>
                    <p className='text-sm leading-relaxed'>{trace.firstUserQuery}</p>
                  </div>
                </div>
                <div className='flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted ring-2 ring-background'>
                  <User className='h-3.5 w-3.5 text-muted-foreground' />
                </div>
              </div>
            )}

            {/* Assistant Response */}
            {trace.firstText && (
              <div className='flex items-start gap-2.5'>
                <div className='flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/10 ring-2 ring-background'>
                  <Bot className='h-3.5 w-3.5 text-primary' />
                </div>
                <div className='flex max-w-[85%] flex-col gap-1'>
                  <div className='relative rounded-2xl rounded-tl-sm bg-muted px-4 py-2.5 text-foreground shadow-sm'>
                    <p className='text-sm leading-relaxed whitespace-pre-wrap'>{trace.firstText}</p>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Footer: Actions */}
          <div className='flex items-center justify-end pt-3'>
            <Button
              variant='ghost'
              size='sm'
              onClick={() => onViewTrace(trace.id)}
              className='h-8 gap-1.5 rounded-lg text-xs font-medium text-muted-foreground/80 transition-colors hover:bg-primary/5 hover:text-primary'
            >
              {t('threads.detail.viewTrace')}
              <ArrowUpRight className='h-3.5 w-3.5 transition-transform group-hover/button:translate-x-0.5 group-hover/button:-translate-y-0.5' />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
