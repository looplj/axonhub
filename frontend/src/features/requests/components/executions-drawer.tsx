import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { Eye, X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { RequestExecution } from '../data/schema'
import { useChannels } from '@/features/channels/data'
import { getStatusColor } from './help'

interface ExecutionsDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  executions: RequestExecution[]
  onExecutionSelect: (execution: RequestExecution) => void
}

export function ExecutionsDrawer({
  open,
  onOpenChange,
  executions,
  onExecutionSelect,
}: ExecutionsDrawerProps) {
  

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className='w-[400px] sm:w-[540px]'>
        <SheetHeader>
          <SheetTitle>执行记录列表</SheetTitle>
        </SheetHeader>
        
        <div className='mt-6'>
          {executions.length > 0 ? (
            <ScrollArea className='h-[calc(100vh-120px)]'>
              <div className='space-y-3'>
                {executions.map((execution, index) => (
                  <div
                    key={execution.id}
                    className='rounded-lg border p-4 hover:bg-gray-50 transition-colors'
                  >
                    <div className='flex items-center justify-between mb-3'>
                      <h4 className='text-sm font-medium'>
                        执行 #{index + 1}
                      </h4>
                      <div className='flex items-center gap-2'>
                        <Badge className={getStatusColor(execution.status)}>
                          {execution.status}
                        </Badge>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => onExecutionSelect(execution)}
                        >
                          <Eye className='mr-2 h-4 w-4' />
                          查看详情
                        </Button>
                      </div>
                    </div>
                    
                    <div className='space-y-2 text-xs text-muted-foreground'>
                      <div className='flex justify-between'>
                        <span>Channel:</span>
                        <span className='font-mono'>{execution.channel?.name || '未知'}</span>
                      </div>
                      <div className='flex justify-between'>
                        <span>模型ID:</span>
                        <span className='font-mono'>{execution.modelID || '未知'}</span>
                      </div>
                      <div className='flex justify-between'>
                        <span>创建时间:</span>
                        <span>
                          {format(
                            new Date(execution.createdAt),
                            'MM-dd HH:mm:ss',
                            { locale: zhCN }
                          )}
                        </span>
                      </div>
                      {execution.errorMessage && (
                        <div className='mt-2 p-2 bg-red-50 rounded text-red-800'>
                          <div className='font-medium'>错误:</div>
                          <div className='truncate'>{execution.errorMessage}</div>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <div className='py-8 text-center'>
              <p className='text-muted-foreground text-sm'>暂无执行记录</p>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}