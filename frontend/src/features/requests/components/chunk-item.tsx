import { JsonViewer } from '@/components/json-tree-view';

interface ChunkItemProps {
  chunk: any;
  index: number;
}

export function ChunkItem({ chunk, index }: ChunkItemProps) {
  return (
    <div className='bg-background rounded-lg border p-4'>
      <div className='flex items-start gap-4'>
        <div className='flex-shrink-0'>
          <span className='text-muted-foreground text-sm font-medium'>Chunk {index + 1}</span>
        </div>
        <div className='min-w-0 flex-1'>
          <JsonViewer data={chunk} rootName='' defaultExpanded={false} className='text-sm' />
        </div>
      </div>
    </div>
  );
}
