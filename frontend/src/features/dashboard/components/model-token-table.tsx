'use client'

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Download, ArrowUpDown } from 'lucide-react'

import { formatNumber } from '@/utils/format-number'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ModelTokenStats } from '../data/dashboard'

interface ModelTokenTableProps {
  currentPeriod: ModelTokenStats[]
  onExport?: () => void
}

type SortField = 'modelId' | 'totalTokens' | 'totalInputTokens' | 'totalOutputTokens' | 'totalCachedTokens'
type SortOrder = 'asc' | 'desc'

export function ModelTokenTable({ currentPeriod, onExport }: ModelTokenTableProps) {
  const { t } = useTranslation()
  const [sortField, setSortField] = useState<SortField>('totalTokens')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')

  const sortedData = [...currentPeriod].sort((a, b) => {
    const aValue = a[sortField]
    const bValue = b[sortField]

    if (typeof aValue === 'string') {
      const comparison = aValue.localeCompare(bValue as string)
      return sortOrder === 'asc' ? comparison : -comparison
    } else {
      const comparison = (aValue as number) - (bValue as number)
      return sortOrder === 'asc' ? comparison : -comparison
    }
  })

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortOrder('desc')
    }
  }

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) {
      return <ArrowUpDown className='ml-2 h-3 w-3' />
    }
    return (
      <ArrowUpDown
        className={`ml-2 h-3 w-3 ${sortOrder === 'asc' ? 'rotate-180' : ''}`}
      />
    )
  }

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div className='text-sm text-muted-foreground'>
          {t('dashboard.stats.showing')} {currentPeriod.length} {t('dashboard.stats.models')}
        </div>
        <Button
          variant='outline'
          size='sm'
          onClick={onExport}
          className='gap-2'
        >
          <Download className='h-4 w-4' />
          {t('common.export')}
        </Button>
      </div>

      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => handleSort('modelId')}
                  className='h-auto p-0 font-medium'
                >
                  {t('dashboard.stats.model')}
                  <SortIcon field='modelId' />
                </Button>
              </TableHead>
              <TableHead className='text-right'>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => handleSort('totalInputTokens')}
                  className='h-auto p-0 font-medium'
                >
                  {t('dashboard.stats.inputTokens')}
                  <SortIcon field='totalInputTokens' />
                </Button>
              </TableHead>
              <TableHead className='text-right'>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => handleSort('totalOutputTokens')}
                  className='h-auto p-0 font-medium'
                >
                  {t('dashboard.stats.outputTokens')}
                  <SortIcon field='totalOutputTokens' />
                </Button>
              </TableHead>
              <TableHead className='text-right'>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => handleSort('totalCachedTokens')}
                  className='h-auto p-0 font-medium'
                >
                  {t('dashboard.stats.cachedTokens')}
                  <SortIcon field='totalCachedTokens' />
                </Button>
              </TableHead>
              <TableHead className='text-right'>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => handleSort('totalTokens')}
                  className='h-auto p-0 font-medium'
                >
                  {t('dashboard.stats.totalTokens')}
                  <SortIcon field='totalTokens' />
                </Button>
              </TableHead>
              <TableHead className='text-right'>
                {t('dashboard.stats.percentage')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedData.map((stat) => {
              const totalTokens = currentPeriod.reduce((sum, s) => sum + s.totalTokens, 0)
              const percentage = totalTokens > 0 ? (stat.totalTokens / totalTokens) * 100 : 0

              return (
                <TableRow key={stat.modelId}>
                  <TableCell className='font-medium'>{stat.modelId}</TableCell>
                  <TableCell className='text-right'>{formatNumber(stat.totalInputTokens)}</TableCell>
                  <TableCell className='text-right'>{formatNumber(stat.totalOutputTokens)}</TableCell>
                  <TableCell className='text-right'>{formatNumber(stat.totalCachedTokens)}</TableCell>
                  <TableCell className='text-right font-semibold'>{formatNumber(stat.totalTokens)}</TableCell>
                  <TableCell className='text-right'>
                    <div className='flex items-center justify-end gap-2'>
                      <span className='text-sm text-muted-foreground'>{percentage.toFixed(1)}%</span>
                      <div className='w-16 bg-secondary rounded-full h-2'>
                        <div
                          className='bg-primary h-2 rounded-full transition-all duration-300'
                          style={{ width: `${Math.min(percentage, 100)}%` }}
                        />
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>

      {/* Summary Statistics */}
      <div className='grid grid-cols-2 md:grid-cols-4 gap-4 mt-4'>
        <div className='rounded-lg border p-3'>
          <div className='text-sm text-muted-foreground'>{t('dashboard.stats.totalInput')}</div>
          <div className='text-xl font-bold'>
            {formatNumber(currentPeriod.reduce((sum, s) => sum + s.totalInputTokens, 0))}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-sm text-muted-foreground'>{t('dashboard.stats.totalOutput')}</div>
          <div className='text-xl font-bold'>
            {formatNumber(currentPeriod.reduce((sum, s) => sum + s.totalOutputTokens, 0))}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-sm text-muted-foreground'>{t('dashboard.stats.totalCached')}</div>
          <div className='text-xl font-bold'>
            {formatNumber(currentPeriod.reduce((sum, s) => sum + s.totalCachedTokens, 0))}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-sm text-muted-foreground'>{t('dashboard.stats.totalTokens')}</div>
          <div className='text-xl font-bold'>
            {formatNumber(currentPeriod.reduce((sum, s) => sum + s.totalTokens, 0))}
          </div>
        </div>
      </div>
    </div>
  )
}