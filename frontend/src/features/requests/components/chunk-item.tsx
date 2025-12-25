import { JsonViewer } from '@/components/json-tree-view'

interface ChunkItemProps {
  chunk: any
  index: number
}

export function ChunkItem({ chunk, index }: ChunkItemProps) {
  return (
    <div className='bg-background rounded-lg border p-4'>
      <div className='flex items-start gap-4'>
        <div className='flex-shrink-0'>
          <span className='text-sm font-medium text-muted-foreground'>
            Chunk {index + 1}
          </span>
        </div>
        <div className='flex-1 min-w-0'>
          <JsonViewer
            data={chunk}
            rootName=''
            defaultExpanded={false}
            className='text-sm'
          />
        </div>
      </div>
    </div>
  )
}
