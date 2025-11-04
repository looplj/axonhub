import { ThumbsUp, ThumbsDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import type { Trace } from '@/features/traces/data/schema'

interface TraceCardProps {
  trace: Trace
  onViewTrace: (traceId: string) => void
}

export function TraceCard({ trace, onViewTrace }: TraceCardProps) {
  return (
    <Card className='border-0 shadow-sm hover:shadow-md transition-shadow'>
      <CardContent className='p-6'>
        <div className='space-y-4'>
          {/* User Query */}
          {trace.firstUserQuery && (
            <div className='flex justify-end'>
              <div className='bg-indigo-50 text-indigo-900 rounded-2xl px-4 py-2 max-w-[80%]'>
                <p className='text-sm'>{trace.firstUserQuery}</p>
              </div>
            </div>
          )}

          {/* Assistant Response */}
          {trace.firstText && (
            <div className='flex justify-start'>
              <div className='bg-gray-50 text-gray-900 rounded-2xl px-4 py-2 max-w-[80%]'>
                <p className='text-sm whitespace-pre-wrap'>{trace.firstText}</p>
              </div>
            </div>
          )}

          {/* Actions */}
          <div className='flex items-center justify-between pt-2'>
            {/* <div className='flex items-center gap-2'>
              <Button variant='ghost' size='sm' className='h-8 w-8 p-0'>
                <ThumbsUp className='h-4 w-4' />
              </Button>
              <Button variant='ghost' size='sm' className='h-8 w-8 p-0'>
                <ThumbsDown className='h-4 w-4' />
              </Button>
            </div> */}
            <Button
              variant='ghost'
              size='sm'
              onClick={() => onViewTrace(trace.id)}
              className='text-sm text-muted-foreground hover:text-foreground'
            >
              View trace
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
