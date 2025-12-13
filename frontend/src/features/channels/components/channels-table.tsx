import React, { useState, useEffect, useMemo, useCallback } from 'react'
import {
  ColumnDef,
  ColumnFiltersState,
  ExpandedState,
  RowData,
  RowSelectionState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { format } from 'date-fns'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { formatDuration } from '@/utils/format-duration'
import { useChannels } from '../context/channels-context'
import { Channel, ChannelConnection } from '../data/schema'
import { CHANNEL_CONFIGS } from '../data/config_channels'
import { DataTableToolbar } from './data-table-toolbar'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface DataTableProps {
  columns: ColumnDef<Channel>[]
  loading?: boolean
  data: Channel[]
  pageInfo?: ChannelConnection['pageInfo']
  pageSize: number
  totalCount?: number
  nameFilter: string
  typeFilter: string[]
  statusFilter: string[]
  tagFilter: string
  modelFilter: string
  selectedTypeTab?: string
  showErrorOnly?: boolean
  onExitErrorOnlyMode?: () => void
  sorting: SortingState
  onSortingChange: (updater: SortingState | ((prev: SortingState) => SortingState)) => void
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onNameFilterChange: (filter: string) => void
  onTypeFilterChange: (filters: string[]) => void
  onStatusFilterChange: (filters: string[]) => void
  onTagFilterChange: (filter: string) => void
  onModelFilterChange: (filter: string) => void
}

export function ChannelsTable({
  columns,
  loading,
  data,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  typeFilter,
  statusFilter,
  tagFilter,
  modelFilter,
  selectedTypeTab = 'all',
  showErrorOnly,
  sorting,
  onSortingChange,
  onExitErrorOnlyMode,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  onTypeFilterChange,
  onStatusFilterChange,
  onTagFilterChange,
  onModelFilterChange,
}: DataTableProps) {
  const { t } = useTranslation()
  const { setSelectedChannels, setResetRowSelection } = useChannels()
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [expanded, setExpanded] = useState<ExpandedState>({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    tags: false, // Hide tags column by default but keep it for filtering
  })
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])

  // Sync server state to local column filters using useEffect
  useEffect(() => {
    const newColumnFilters: ColumnFiltersState = []

    if (nameFilter) {
      newColumnFilters.push({ id: 'name', value: nameFilter })
    }
    if (typeFilter.length > 0) {
      newColumnFilters.push({ id: 'provider', value: typeFilter })
    }
    if (statusFilter.length > 0) {
      newColumnFilters.push({ id: 'status', value: statusFilter })
    }
    if (tagFilter) {
      newColumnFilters.push({ id: 'tags', value: tagFilter })
    }
    if (modelFilter) {
      newColumnFilters.push({ id: 'model', value: modelFilter })
    }

    setColumnFilters(newColumnFilters)
  }, [nameFilter, typeFilter, statusFilter, tagFilter, modelFilter])

  // Handle column filter changes and sync with server
  const handleColumnFiltersChange = (
    updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)
  ) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)

    // Extract filter values
    const nameFilterValue = newFilters.find((filter) => filter.id === 'name')?.value as string
    const typeFilterValue = newFilters.find((filter) => filter.id === 'provider')?.value as string[]
    const statusFilterValue = newFilters.find((filter) => filter.id === 'status')?.value as string[]
    const tagFilterValue = newFilters.find((filter) => filter.id === 'tags')?.value as string
    const modelFilterValue = newFilters.find((filter) => filter.id === 'model')?.value as string

    // Update server filters only if changed
    const newNameFilter = nameFilterValue || ''
    const newTypeFilter = Array.isArray(typeFilterValue) ? typeFilterValue : []
    const newStatusFilter = Array.isArray(statusFilterValue) ? statusFilterValue : []
    const newTagFilter = tagFilterValue || ''
    const newModelFilter = modelFilterValue || ''

    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter)
    }

    if (JSON.stringify(newTypeFilter.sort()) !== JSON.stringify(typeFilter.sort())) {
      onTypeFilterChange(newTypeFilter)
    }

    if (JSON.stringify(newStatusFilter.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(newStatusFilter)
    }

    if (newTagFilter !== tagFilter) {
      onTagFilterChange(newTagFilter)
    }

    if (newModelFilter !== modelFilter) {
      onModelFilterChange(newModelFilter)
    }
  }

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
      expanded,
    },
    enableRowSelection: true,
    getRowId: (row) => row.id,
    onRowSelectionChange: setRowSelection,
    onExpandedChange: setExpanded,
    onSortingChange,
    onColumnFiltersChange: handleColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    getExpandedRowModel: getExpandedRowModel(),
    // Enable server-side pagination and filtering
    manualPagination: true,
    manualFiltering: true, // Enable manual filtering for server-side filtering
  })

  const filteredSelectedRows = useMemo(
    () => table.getFilteredSelectedRowModel().rows,
    [table, rowSelection, data]
  )

  const getApiFormatLabel = useCallback(
    (apiFormat?: string) => {
      if (!apiFormat) return '-'

      const key = `channels.dialogs.fields.apiFormat.formats.${apiFormat}`
      const label = t(key)
      return label === key ? apiFormat : label
    },
    [t]
  )
  const selectedCount = filteredSelectedRows.length
  const isFiltered = columnFilters.length > 0

  useEffect(() => {
    const resetFn = () => {
      setRowSelection({})
    }
    setResetRowSelection(resetFn)
  }, [setResetRowSelection])

  useEffect(() => {
    const selected = filteredSelectedRows.map((row) => row.original as Channel)
    setSelectedChannels(selected)
  }, [filteredSelectedRows, setSelectedChannels])

  useEffect(() => {
    if (selectedCount === 0) {
      setSelectedChannels([])
    }
  }, [selectedCount, setSelectedChannels])

  // Clear rowSelection when data changes and selected rows no longer exist
  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((channel) => channel.id))
      const selectedIds = Object.keys(rowSelection)
      const anySelectedIdMissing = selectedIds.some((id) => !dataIds.has(id))
      
      if (anySelectedIdMissing) {
        // Some selected rows no longer exist in the new data, clear selection
        setRowSelection({})
      }
    }
  }, [data, rowSelection])

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar 
        table={table} 
        isFiltered={isFiltered} 
        selectedCount={selectedCount}
        selectedTypeTab={selectedTypeTab}
        showErrorOnly={showErrorOnly}
        onExitErrorOnlyMode={onExitErrorOnlyMode}
      />
      <div className='mt-4 flex-1 overflow-auto rounded-md border relative'>
        <Table data-testid='channels-table'>
          <TableHeader className='bg-background sticky top-0 z-10'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row'>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
                      className={header.column.columnDef.meta?.className ?? ''}
                    >
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => {
                const channel = row.original
                const config = CHANNEL_CONFIGS[channel.type]
                const performance = channel.channelPerformance
                return (
                  <React.Fragment key={row.id}>
                    <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'} className='group/row'>
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id} className={cell.column.columnDef.meta?.className ?? ''}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      ))}
                    </TableRow>
                    {row.getIsExpanded() && (
                      <TableRow key={`${row.id}-expanded`} className='bg-muted/30 hover:bg-muted/50'>
                        <TableCell colSpan={columns.length} className='p-6 whitespace-normal'>
                          <div className='space-y-6'>
                            {/* Top Section: Basic Info (left) + Additional Info & Performance (right, stacked) */}
                            <div className='grid grid-cols-1 gap-6 md:grid-cols-2'>
                              {/* Basic Info */}
                              <div className='space-y-3'>
                                <h4 className='text-sm font-semibold'>{t('channels.expandedRow.basic')}</h4>
                                <div className='space-y-2 text-sm'>
                                  <div className='flex items-start gap-2'>
                                    <span className='text-muted-foreground shrink-0'>
                                      {t('channels.columns.baseURL')}:
                                    </span>
                                    <span className='flex-1 min-w-0 font-mono text-xs break-all text-right'>{channel.baseURL}</span>
                                  </div>
                                  <div className='flex justify-between items-center'>
                                    <span className='text-muted-foreground'>{t('channels.columns.type')}:</span>
                                    <Badge variant='outline' className={config?.color}>
                                      {t(`channels.types.${channel.type}`)}
                                    </Badge>
                                  </div>
                                  <div className='flex justify-between items-center'>
                                    <span className='text-muted-foreground'>{t('channels.expandedRow.apiFormat')}:</span>
                                    <span className='font-mono text-xs'>{getApiFormatLabel(config?.apiFormat)}</span>
                                  </div>
                                  <div className='flex justify-between'>
                                    <span className='text-muted-foreground'>{t('channels.columns.createdAt')}:</span>
                                    <span>{format(channel.createdAt, 'yyyy-MM-dd HH:mm')}</span>
                                  </div>
                                  <div className='flex justify-between'>
                                    <span className='text-muted-foreground'>{t('channels.columns.updatedAt')}:</span>
                                    <span>{format(channel.updatedAt, 'yyyy-MM-dd HH:mm')}</span>
                                  </div>
                                </div>
                              </div>

                              {/* Right Side: Additional Info (top) + Performance (bottom) */}
                              <div className='space-y-6'>
                                {/* Additional Info */}
                                <div className='space-y-3'>
                                  <h4 className='text-sm font-semibold'>{t('channels.expandedRow.additional')}</h4>
                                  <div className='space-y-2 text-sm'>
                                    <div className='flex justify-between items-center'>
                                      <span className='text-muted-foreground'>{t('channels.columns.weight')}:</span>
                                      <span className='font-mono text-xs'>{channel.orderingWeight ?? 0}</span>
                                    </div>
                                    <div className='flex justify-between'>
                                      <span className='text-muted-foreground'>{t('channels.expandedRow.remark')}:</span>
                                      <span className='max-w-[200px] truncate text-right' title={channel.remark || undefined}>
                                        {channel.remark || '-'}
                                      </span>
                                    </div>
                                    <div className='flex justify-between items-start'>
                                      <span className='text-muted-foreground shrink-0'>{t('channels.expandedRow.tags')}:</span>
                                      <div className='flex flex-wrap gap-1 justify-end max-w-[200px]'>
                                        {channel.tags && channel.tags.length > 0 ? (
                                          channel.tags.map((tag) => (
                                            <Badge key={tag} variant='outline' className='text-xs'>
                                              {tag}
                                            </Badge>
                                          ))
                                        ) : (
                                          <span>-</span>
                                        )}
                                      </div>
                                    </div>
                                  </div>
                                </div>

                                {/* Performance */}
                                <div className='space-y-3'>
                                  <h4 className='text-sm font-semibold'>{t('channels.expandedRow.performance')}</h4>
                                  <div className='space-y-2 text-sm'>
                                    {performance ? (
                                      <>
                                        <div className='flex justify-between'>
                                          <span className='text-muted-foreground'>{t('channels.columns.firstTokenLatencyFull')}:</span>
                                          <span>{formatDuration(performance.avgStreamFirstTokenLatencyMs || performance.avgLatencyMs || 0)}</span>
                                        </div>
                                        <div className='flex justify-between'>
                                          <span className='text-muted-foreground'>{t('channels.columns.tokensPerSecondFull')}:</span>
                                          <span>{(performance.avgStreamTokenPerSecond || performance.avgTokenPerSecond || 0).toFixed(1)}</span>
                                        </div>
                                      </>
                                    ) : (
                                      <span className='text-muted-foreground'>{t('channels.expandedRow.noPerformanceData')}</span>
                                    )}
                                  </div>
                                </div>
                              </div>
                            </div>

                            {/* Bottom Section: Model Info (single column, full width) */}
                            <div className='space-y-3 border-t pt-4'>
                              <h4 className='text-sm font-semibold'>{t('channels.expandedRow.modes')}</h4>
                              <div className='space-y-3'>
                                <div className='flex items-center gap-6 text-sm'>
                                  <div className='flex items-center gap-2'>
                                    <span className='text-muted-foreground'>{t('channels.expandedRow.totalModels')}:</span>
                                    <span className='font-medium'>{channel.supportedModels.length}</span>
                                  </div>
                                  <div className='flex items-center gap-2'>
                                    <span className='text-muted-foreground'>{t('channels.expandedRow.defaultTestModel')}:</span>
                                    <span className='font-medium'>{channel.defaultTestModel || '-'}</span>
                                  </div>
                                </div>
                                <div className='flex flex-wrap gap-1'>
                                  {channel.supportedModels.slice(0, 20).map((model) => (
                                    <Badge key={model} variant='secondary' className='text-xs'>
                                      {model}
                                    </Badge>
                                  ))}
                                  {channel.supportedModels.length > 20 && (
                                    <Badge variant='outline' className='text-xs'>
                                      +{channel.supportedModels.length - 20} {t('channels.expandedRow.more')}
                                    </Badge>
                                  )}
                                </div>
                              </div>
                            </div>
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </React.Fragment>
                )
              })
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
                  {t('common.noData')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className='mt-4 flex-shrink-0'>
        <ServerSidePagination
          pageInfo={pageInfo}
          pageSize={pageSize}
          dataLength={data.length}
          totalCount={totalCount}
          selectedRows={selectedCount}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
    </div>
  )
}
