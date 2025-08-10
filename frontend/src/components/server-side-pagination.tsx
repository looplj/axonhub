'use client'

import {
  ChevronLeftIcon,
  ChevronRightIcon,
  DoubleArrowLeftIcon,
  DoubleArrowRightIcon,
} from '@radix-ui/react-icons'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface PageInfo {
  hasNextPage: boolean
  hasPreviousPage: boolean
  startCursor?: string | null
  endCursor?: string | null
}

interface ServerSidePaginationProps {
  pageInfo?: PageInfo
  pageSize: number
  dataLength: number
  totalCount?: number
  selectedRows: number
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
}

export function ServerSidePagination({
  pageInfo,
  pageSize,
  dataLength,
  totalCount,
  selectedRows,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
}: ServerSidePaginationProps) {
  const { t } = useTranslation()
  
  return (
    <div
      className='flex items-center justify-between overflow-clip px-2'
      style={{ overflowClipMargin: 1 }}
    >
      <div className='text-muted-foreground hidden flex-1 text-sm sm:block'>
        {totalCount !== undefined 
          ? t('pagination.selectedInfoWithTotal', { selectedRows, dataLength, totalCount })
          : t('pagination.selectedInfo', { selectedRows, dataLength })
        }
      </div>
      <div className='flex items-center sm:space-x-6 lg:space-x-8'>
        <div className='flex items-center space-x-2'>
          <p className='hidden text-sm font-medium sm:block'>{t('pagination.rowsPerPage')}</p>
          <Select
            value={`${pageSize}`}
            onValueChange={(value) => {
              onPageSizeChange(Number(value))
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
            <span className='text-muted-foreground'>
              {pageInfo?.hasNextPage ? t('pagination.hasNext') : t('pagination.lastPage')}
            </span>
          </div>
        </div>
        <div className='flex items-center space-x-2'>
          <Button
            variant='outline'
            className='hidden h-8 w-8 p-0 lg:flex'
            onClick={onPreviousPage}
            disabled={!pageInfo?.hasPreviousPage}
          >
            <span className='sr-only'>{t('pagination.firstPage')}</span>
            <DoubleArrowLeftIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className='h-8 w-8 p-0'
            onClick={onPreviousPage}
            disabled={!pageInfo?.hasPreviousPage}
          >
            <span className='sr-only'>{t('pagination.previousPage')}</span>
            <ChevronLeftIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className='h-8 w-8 p-0'
            onClick={onNextPage}
            disabled={!pageInfo?.hasNextPage}
          >
            <span className='sr-only'>{t('pagination.nextPage')}</span>
            <ChevronRightIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className='hidden h-8 w-8 p-0 lg:flex'
            onClick={onNextPage}
            disabled={!pageInfo?.hasNextPage}
          >
            <span className='sr-only'>{t('pagination.lastPage')}</span>
            <DoubleArrowRightIcon className='h-4 w-4' />
          </Button>
        </div>
      </div>
    </div>
  )
}