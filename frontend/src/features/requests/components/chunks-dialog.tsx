import { useState, useCallback, useMemo, useEffect } from 'react';
import { ChevronLeftIcon, ChevronRightIcon, DoubleArrowLeftIcon, DoubleArrowRightIcon } from '@radix-ui/react-icons';
import { Layers } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ChunkItem } from './chunk-item';

interface ChunksDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  chunks: any[];
  title?: string;
}

const PAGE_SIZE_OPTIONS = [10, 20, 30, 50];
const DEFAULT_PAGE_SIZE = 20;

export function ChunksDialog({ open, onOpenChange, chunks, title }: ChunksDialogProps) {
  const { t } = useTranslation();
  const [chunksPage, setChunksPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [pageInputValue, setPageInputValue] = useState('1');

  // Pagination logic for chunks
  const paginatedChunks = useMemo(() => {
    const startIndex = (chunksPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    return chunks.slice(startIndex, endIndex);
  }, [chunks, chunksPage, pageSize]);

  const totalChunksPages = useMemo(() => {
    return Math.ceil(chunks.length / pageSize);
  }, [chunks.length, pageSize]);

  const handleChunksPageChange = useCallback((newPage: number) => {
    setChunksPage(newPage);
    setPageInputValue(String(newPage));
  }, []);

  const handlePageSizeChange = useCallback((newSize: number) => {
    setPageSize(newSize);
    setChunksPage(1);
    setPageInputValue('1');
  }, []);

  const handlePageInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setPageInputValue(e.target.value);
  }, []);

  const handlePageInputBlur = useCallback(() => {
    const page = parseInt(pageInputValue, 10);
    if (!isNaN(page) && page >= 1 && page <= totalChunksPages) {
      setChunksPage(page);
    } else {
      setPageInputValue(String(chunksPage));
    }
  }, [pageInputValue, totalChunksPages, chunksPage]);

  const handlePageInputKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handlePageInputBlur();
    }
  }, [handlePageInputBlur]);

  // Reset page when chunks change
  useEffect(() => {
    if (open && chunks.length > 0) {
      setChunksPage(1);
      setPageInputValue('1');
    }
  }, [open, chunks.length]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[80vh] flex-col sm:max-w-4xl'>
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
            <div className='bg-muted/20 w-full flex-1 overflow-auto rounded-lg border p-4'>
              <div className='space-y-4'>
                {paginatedChunks.map((chunk, index) => (
                  <ChunkItem
                    key={(chunksPage - 1) * pageSize + index}
                    chunk={chunk}
                    index={(chunksPage - 1) * pageSize + index}
                  />
                ))}
              </div>
            </div>

            {/* Pagination Controls */}
            {totalChunksPages > 1 && (
              <div className='flex items-center justify-between border-t pt-4'>
                <div className='text-muted-foreground flex-1 text-sm'>
                  {t('pagination.showing', {
                    start: (chunksPage - 1) * pageSize + 1,
                    end: Math.min(chunksPage * pageSize, chunks.length),
                    total: chunks.length,
                  })}
                </div>
                <div className='flex items-center space-x-6'>
                  <div className='flex items-center space-x-2'>
                    <p className='text-sm font-medium'>{t('pagination.rowsPerPage')}</p>
                    <Select value={`${pageSize}`} onValueChange={(value) => handlePageSizeChange(Number(value))}>
                      <SelectTrigger className='h-8 w-[70px]'>
                        <SelectValue placeholder={pageSize} />
                      </SelectTrigger>
                      <SelectContent side='top'>
                        {PAGE_SIZE_OPTIONS.map((size) => (
                          <SelectItem key={size} value={`${size}`}>
                            {size}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <Input
                      className='h-8 w-12 text-center'
                      value={pageInputValue}
                      onChange={handlePageInputChange}
                      onBlur={handlePageInputBlur}
                      onKeyDown={handlePageInputKeyDown}
                    />
                    <span className='text-muted-foreground text-sm'>/ {totalChunksPages}</span>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <Button
                      variant='outline'
                      className='h-8 w-8 p-0'
                      onClick={() => handleChunksPageChange(1)}
                      disabled={chunksPage === 1}
                    >
                      <span className='sr-only'>{t('pagination.firstPage')}</span>
                      <DoubleArrowLeftIcon className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='outline'
                      className='h-8 w-8 p-0'
                      onClick={() => handleChunksPageChange(chunksPage - 1)}
                      disabled={chunksPage === 1}
                    >
                      <span className='sr-only'>{t('pagination.previousPage')}</span>
                      <ChevronLeftIcon className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='outline'
                      className='h-8 w-8 p-0'
                      onClick={() => handleChunksPageChange(chunksPage + 1)}
                      disabled={chunksPage === totalChunksPages}
                    >
                      <span className='sr-only'>{t('pagination.nextPage')}</span>
                      <ChevronRightIcon className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='outline'
                      className='h-8 w-8 p-0'
                      onClick={() => handleChunksPageChange(totalChunksPages)}
                      disabled={chunksPage === totalChunksPages}
                    >
                      <span className='sr-only'>{t('pagination.lastPage')}</span>
                      <DoubleArrowRightIcon className='h-4 w-4' />
                    </Button>
                  </div>
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
  );
}
