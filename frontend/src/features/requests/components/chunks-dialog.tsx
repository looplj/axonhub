import { useState, useCallback, useMemo, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Layers, ChevronLeft, ChevronRight } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ChunkItem } from './chunk-item'

interface ChunksDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  chunks: any[]
  title?: string
}

const CHUNKS_PER_PAGE = 20

export function ChunksDialog({ open, onOpenChange, chunks, title }: ChunksDialogProps) {
  const { t } = useTranslation()
  const [chunksPage, setChunksPage] = useState(1)

  // Pagination logic for chunks
  const paginatedChunks = useMemo(() => {
    const startIndex = (chunksPage - 1) * CHUNKS_PER_PAGE
    const endIndex = startIndex + CHUNKS_PER_PAGE
    return chunks.slice(startIndex, endIndex)
  }, [chunks, chunksPage])

  const totalChunksPages = useMemo(() => {
    return Math.ceil(chunks.length / CHUNKS_PER_PAGE)
  }, [chunks.length])

  const handleChunksPageChange = useCallback((newPage: number) => {
    setChunksPage(newPage)
  }, [])

  // Reset page when chunks change
  useEffect(() => {
    if (open && chunks.length > 0) {
      setChunksPage(1)
    }
  }, [open, chunks.length])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-4xl max-h-[80vh] flex flex-col'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Layers className='h-5 w-5' />
            {title || t('requests.dialogs.jsonViewer.responseChunks')}
            <Badge variant='secondary' className='ml-2'>
              {chunks.length} {t('requests.columns.responseChunks')}
            </Badge>
          </DialogTitle>
        </DialogHeader>
        
        {chunks.length > 0 ? (
          <>
            <div className='bg-muted/20 flex-1 w-full overflow-auto rounded-lg border p-4'>
              <div className='space-y-4'>
                {paginatedChunks.map((chunk, index) => (
                  <ChunkItem 
                    key={(chunksPage - 1) * CHUNKS_PER_PAGE + index} 
                    chunk={chunk} 
                    index={(chunksPage - 1) * CHUNKS_PER_PAGE + index}
                  />
                ))}
              </div>
            </div>
            
            {/* Pagination Controls */}
            {totalChunksPages > 1 && (
              <div className='flex items-center justify-between pt-4 border-t'>
                <div className='text-sm text-muted-foreground'>
                  {t('pagination.showing', {
                    start: (chunksPage - 1) * CHUNKS_PER_PAGE + 1,
                    end: Math.min(chunksPage * CHUNKS_PER_PAGE, chunks.length),
                    total: chunks.length
                  })}
                </div>
                <div className='flex items-center gap-2'>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => handleChunksPageChange(chunksPage - 1)}
                    disabled={chunksPage === 1}
                  >
                    <ChevronLeft className='h-4 w-4' />
                    {t('pagination.previousPage')}
                  </Button>
                  <span className='text-sm font-medium px-3 py-1'>
                    {chunksPage} / {totalChunksPages}
                  </span>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => handleChunksPageChange(chunksPage + 1)}
                    disabled={chunksPage === totalChunksPages}
                  >
                    {t('pagination.nextPage')}
                    <ChevronRight className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            )}
          </>
        ) : (
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-3 text-center'>
              <Layers className='text-muted-foreground mx-auto h-12 w-12' />
              <p className='text-muted-foreground text-base'>{t('requests.detail.noResponse')}</p>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
