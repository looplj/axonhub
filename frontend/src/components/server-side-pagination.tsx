'use client';

import { ChevronLeftIcon, ChevronRightIcon, DoubleArrowLeftIcon } from '@radix-ui/react-icons';
import type { PageInfo } from '@/gql/pagination';
import { useTranslation } from 'react-i18next';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

interface ServerSidePaginationProps {
  pageInfo?: PageInfo;
  pageSize: number;
  dataLength: number;
  totalCount?: number;
  selectedRows: number;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onFirstPage?: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onResetCursor?: () => void;
}

export function ServerSidePagination({
  pageInfo,
  pageSize,
  dataLength,
  totalCount,
  selectedRows,
  onNextPage,
  onPreviousPage,
  onFirstPage,
  onPageSizeChange,
  onResetCursor,
}: ServerSidePaginationProps) {
  const { t } = useTranslation();
  const { resetCursor } = usePaginationSearch({
    defaultPageSize: 20,
  });

  return (
    <div className='flex items-center justify-between overflow-clip px-2' style={{ overflowClipMargin: 1 }}>
      <div className='text-muted-foreground hidden flex-1 text-sm sm:block'>
        {totalCount !== undefined
          ? t('pagination.selectedInfoWithTotal', { selectedRows, dataLength, totalCount })
          : t('pagination.selectedInfo', { selectedRows, dataLength })}
      </div>
      <div className='flex items-center sm:space-x-6 lg:space-x-8'>
        <div className='flex items-center space-x-2'>
          <p className='hidden text-sm font-medium sm:block'>{t('pagination.rowsPerPage')}</p>
          <Select
            value={`${pageSize}`}
            onValueChange={(value) => {
              onPageSizeChange(Number(value));
            }}
          >
            <SelectTrigger className='h-8 w-[70px]'>
              <SelectValue placeholder={pageSize} />
            </SelectTrigger>
            <SelectContent side='top'>
              {[10, 20, 30, 40, 50].map((size) => (
                <SelectItem key={size} value={`${size}`}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='flex items-center justify-center text-sm font-medium'>
          <div className='flex items-center space-x-1'>
            <span className='text-muted-foreground'>
              {pageInfo?.hasPreviousPage ? t('pagination.hasPrevious') : t('pagination.firstPage')}
            </span>
            <span className='text-muted-foreground'>|</span>
            <span className='text-muted-foreground'>{pageInfo?.hasNextPage ? t('pagination.hasNext') : t('pagination.lastPage')}</span>
          </div>
        </div>
        <div className='flex items-center space-x-2'>
          <Button
            variant='outline'
            className='hidden h-8 w-8 p-0 lg:flex'
            onClick={onFirstPage || onResetCursor || resetCursor}
            disabled={!pageInfo?.hasPreviousPage}
          >
            <span className='sr-only'>{t('pagination.firstPage')}</span>
            <DoubleArrowLeftIcon className='h-4 w-4' />
          </Button>
          <Button variant='outline' className='h-8 w-8 p-0' onClick={onPreviousPage} disabled={!pageInfo?.hasPreviousPage}>
            <span className='sr-only'>{t('pagination.previousPage')}</span>
            <ChevronLeftIcon className='h-4 w-4' />
          </Button>
          <Button variant='outline' className='h-8 w-8 p-0' onClick={onNextPage} disabled={!pageInfo?.hasNextPage}>
            <span className='sr-only'>{t('pagination.nextPage')}</span>
            <ChevronRightIcon className='h-4 w-4' />
          </Button>
          {/* NOT SUPPORTED */}
          {/* <Button
            variant='outline'
            className='hidden h-8 w-8 p-0 lg:flex'
            onClick={onNextPage}
            disabled={!pageInfo?.hasNextPage}
          >
            <span className='sr-only'>{t('pagination.lastPage')}</span>
            <DoubleArrowRightIcon className='h-4 w-4' />
          </Button> */}
        </div>
      </div>
    </div>
  );
}
