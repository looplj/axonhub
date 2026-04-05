import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { format } from 'date-fns';
import {
  ColumnDef,
  ColumnFiltersState,
  ExpandedState,
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
} from '@tanstack/react-table';
import { motion, AnimatePresence } from 'framer-motion';
import { IconBan, IconCheck, IconX, IconTrash, IconChevronDown, IconChevronRight, IconChevronsDown, IconChevronsUp } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { PermissionGuard } from '@/components/permission-guard';
import { useModels } from '../context/models-context';
import { Model } from '../data/schema';

const MotionTableRow = motion.create(TableRow);

export function useDeveloperLabel() {
  const { t, i18n } = useTranslation();
  return useCallback(
    (developer: string) => {
      const key = `models.developers.${developer}`;
      return i18n.exists(key) ? t(key) : developer;
    },
    [t, i18n]
  );
}

interface ModelsTableProps {
  columns: ColumnDef<Model>[];
  data: Model[];
  loading?: boolean;
  totalCount?: number;
  nameFilter: string;
  sorting: SortingState;
  onSortingChange: (updater: SortingState | ((prev: SortingState) => SortingState)) => void;
  onNameFilterChange: (filter: string) => void;
  canWrite?: boolean;
}

export function ModelsTable({
  columns,
  data,
  loading,
  totalCount,
  nameFilter,
  sorting,
  onSortingChange,
  onNameFilterChange,
  canWrite = true,
}: ModelsTableProps) {
  const { t } = useTranslation();
  const getDeveloperLabel = useDeveloperLabel();
  const { setSelectedModels, setResetRowSelection, setOpen } = useModels();
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [expanded, setExpanded] = useState<ExpandedState>({});
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());

  useEffect(() => {
    const newColumnFilters: ColumnFiltersState = [];
    if (nameFilter) {
      newColumnFilters.push({ id: 'name', value: nameFilter });
    }
    setColumnFilters(newColumnFilters);
  }, [nameFilter]);

  const handleColumnFiltersChange = (updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater;
    setColumnFilters(newFilters);

    const nameFilterValue = newFilters.find((filter) => filter.id === 'name')?.value as string;
    const newNameFilter = nameFilterValue || '';

    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter);
    }
  };

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
    manualFiltering: true,
  });

  // Group rows by developer
  const groupedRows = useMemo(() => {
    const rows = table.getRowModel().rows;
    const groups = new Map<string, typeof rows>();
    rows.forEach((row) => {
      const developer = row.original.developer || 'Unknown';
      if (!groups.has(developer)) groups.set(developer, []);
      groups.get(developer)!.push(row);
    });
    return new Map([...groups.entries()].sort(([a], [b]) => a.localeCompare(b)));
  }, [table, data, sorting]);

  const allGroupsCollapsed = groupedRows.size > 0 && collapsedGroups.size === groupedRows.size;

  const toggleGroup = useCallback((developer: string) => {
    setCollapsedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(developer)) {
        next.delete(developer);
      } else {
        next.add(developer);
      }
      return next;
    });
  }, []);

  const toggleAllGroups = useCallback(() => {
    if (allGroupsCollapsed) {
      setCollapsedGroups(new Set());
    } else {
      setCollapsedGroups(new Set(groupedRows.keys()));
    }
  }, [allGroupsCollapsed, groupedRows]);

  const filteredSelectedRows = useMemo(() => table.getFilteredSelectedRowModel().rows, [table, rowSelection, data]);

  const selectedCount = filteredSelectedRows.length;

  useEffect(() => {
    const resetFn = () => {
      setRowSelection({});
    };
    setResetRowSelection(resetFn);
  }, [setResetRowSelection]);

  useEffect(() => {
    const selected = filteredSelectedRows.map((row) => row.original as Model);
    setSelectedModels(selected);
  }, [filteredSelectedRows, setSelectedModels]);

  useEffect(() => {
    if (selectedCount === 0) {
      setSelectedModels([]);
    }
  }, [selectedCount, setSelectedModels]);

  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((model) => model.id));
      const selectedIds = Object.keys(rowSelection);
      const anySelectedIdMissing = selectedIds.some((id) => !dataIds.has(id));

      if (anySelectedIdMissing) {
        setRowSelection({});
      }
    }
  }, [data, rowSelection]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <div className='mb-4 flex items-center justify-between'>
        <div className='flex flex-1 items-center space-x-2'>
          <Input
            placeholder={t('models.filters.filterByName')}
            value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
            onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
            className='h-8 w-[150px] lg:w-[250px]'
          />
        </div>
        <div className='flex items-center gap-2'>
          {!loading && data.length > 0 && (
            <>
              <span className='text-muted-foreground text-sm'>
                {t('models.groupedView.summary', { groups: groupedRows.size, models: totalCount ?? data.length })}
              </span>
              <Button variant='outline' size='sm' className='h-8' onClick={toggleAllGroups}>
                {allGroupsCollapsed ? (
                  <IconChevronsDown className='mr-1 h-4 w-4' />
                ) : (
                  <IconChevronsUp className='mr-1 h-4 w-4' />
                )}
                {allGroupsCollapsed ? t('models.groupedView.expandAll') : t('models.groupedView.collapseAll')}
              </Button>
            </>
          )}
        </div>
      </div>

      <div className='shadow-soft relative mt-4 flex-1 overflow-auto rounded-2xl border border-[var(--table-border)]'>
        <div className='min-w-max'>
        <Table data-testid='models-table' className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
          <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row border-0'>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
                      className={`${header.column.columnDef.meta?.className ?? ''} text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase`}
                    >
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody className='space-y-1 !bg-[var(--table-background)] p-2'>
            {loading ? (
              <TableSkeleton rows={15} columns={columns.length} />
            ) : groupedRows.size > 0 ? (
              Array.from(groupedRows.entries()).map(([developer, rows]) => {
                const isCollapsed = collapsedGroups.has(developer);
                return (
                  <React.Fragment key={developer}>
                    {/* Developer group header */}
                    <TableRow
                      className='bg-muted/40 hover:bg-muted/60 cursor-pointer border-0 transition-colors'
                      onClick={() => toggleGroup(developer)}
                    >
                      <TableCell colSpan={columns.length} className='border-0 px-4 py-2.5'>
                        <div className='flex items-center gap-2'>
                          {isCollapsed ? (
                            <IconChevronRight className='h-4 w-4 shrink-0' />
                          ) : (
                            <IconChevronDown className='h-4 w-4 shrink-0' />
                          )}
                          <span className='text-sm font-semibold'>{getDeveloperLabel(developer)}</span>
                          <Badge variant='secondary' className='text-xs'>
                            {rows.length}
                          </Badge>
                        </div>
                      </TableCell>
                    </TableRow>
                    {/* Model rows within group */}
                    {!isCollapsed &&
                      rows.map((row) => {
                        const model = row.original;
                        const modelCard = model.modelCard;
                        return (
                          <React.Fragment key={row.id}>
                            <MotionTableRow
                              key={row.id}
                              data-state={row.getIsSelected() && 'selected'}
                              className='group/row table-row-hover rounded-xl border-0 !bg-[var(--table-background)] transition-all duration-200 ease-in-out'
                            >
                              {row.getVisibleCells().map((cell) => (
                                <TableCell key={cell.id} className={`${cell.column.columnDef.meta?.className ?? ''} border-0 bg-inherit px-4 py-3`}>
                                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                                </TableCell>
                              ))}
                            </MotionTableRow>
                            <AnimatePresence>
                              {row.getIsExpanded() && (
                                <TableRow key={`${row.id}-expanded`} className='border-0'>
                                  <TableCell colSpan={columns.length} className='p-0 border-0'>
                                    <motion.div
                                      initial={{ height: 0, opacity: 0 }}
                                      animate={{ height: 'auto', opacity: 1 }}
                                      exit={{ height: 0, opacity: 0 }}
                                      transition={{ duration: 0.2, ease: 'easeInOut' }}
                                      className='bg-muted/30 p-6 hover:bg-muted/50'
                                    >
                                      <div className='space-y-6'>
                                    {/* Top Section: Basic Info (left) + Capabilities (right) */}
                                    <div className='grid grid-cols-1 gap-6 md:grid-cols-2'>
                                      {/* Basic Info */}
                                      <div className='space-y-3'>
                                        <h4 className='text-sm font-semibold'>{t('models.expandedRow.basic')}</h4>
                                        <div className='space-y-2 text-sm'>
                                          <div className='flex justify-between'>
                                            <span className='text-muted-foreground'>{t('models.columns.modelId')}:</span>
                                            <span className='font-mono text-xs'>{model.modelID}</span>
                                          </div>
                                          <div className='flex items-center justify-between'>
                                            <span className='text-muted-foreground'>{t('models.columns.developer')}:</span>
                                            <Badge variant='outline'>{getDeveloperLabel(model.developer)}</Badge>
                                          </div>
                                          <div className='flex items-center justify-between'>
                                            <span className='text-muted-foreground'>{t('models.columns.group')}:</span>
                                            <span>{model.group}</span>
                                          </div>
                                          <div className='flex justify-between'>
                                            <span className='text-muted-foreground'>{t('common.columns.createdAt')}:</span>
                                            <span>{format(model.createdAt, 'yyyy-MM-dd HH:mm')}</span>
                                          </div>
                                          <div className='flex justify-between'>
                                            <span className='text-muted-foreground'>{t('common.columns.updatedAt')}:</span>
                                            <span>{format(model.updatedAt, 'yyyy-MM-dd HH:mm')}</span>
                                          </div>
                                          {model.remark && (
                                            <div className='flex justify-between'>
                                              <span className='text-muted-foreground'>{t('models.columns.remark')}:</span>
                                              <span className='max-w-[200px] truncate text-right' title={model.remark}>
                                                {model.remark}
                                              </span>
                                            </div>
                                          )}
                                        </div>
                                      </div>

                                      {/* Capabilities */}
                                      <div className='space-y-3'>
                                        <h4 className='text-sm font-semibold'>{t('models.expandedRow.capabilities')}</h4>
                                        <div className='space-y-2 text-sm'>
                                          <div className='flex items-center justify-between'>
                                            <span className='text-muted-foreground'>{t('models.modelCard.toolCall')}:</span>
                                            <span>{modelCard?.toolCall ? <IconCheck className='h-4 w-4 text-green-600' /> : '-'}</span>
                                          </div>
                                          <div className='flex items-center justify-between'>
                                            <span className='text-muted-foreground'>{t('models.modelCard.vision')}:</span>
                                            <span>{modelCard?.vision ? <IconCheck className='h-4 w-4 text-green-600' /> : '-'}</span>
                                          </div>
                                          <div className='flex items-center justify-between'>
                                            <span className='text-muted-foreground'>{t('models.modelCard.temperature')}:</span>
                                            <span>{modelCard?.temperature ? <IconCheck className='h-4 w-4 text-green-600' /> : '-'}</span>
                                          </div>
                                          {/* Reasoning grouped */}
                                          <div className='space-y-1'>
                                            <span className='text-muted-foreground'>{t('models.modelCard.reasoning')}:</span>
                                            <div className='ml-4 space-y-1'>
                                              <div className='flex items-center justify-between'>
                                                <span className='text-muted-foreground text-xs'>{t('models.modelCard.reasoningSupported')}:</span>
                                                <span>
                                                  {modelCard?.reasoning?.supported ? <IconCheck className='h-4 w-4 text-green-600' /> : '-'}
                                                </span>
                                              </div>
                                              <div className='flex items-center justify-between'>
                                                <span className='text-muted-foreground text-xs'>{t('models.modelCard.reasoningDefault')}:</span>
                                                <span>
                                                  {modelCard?.reasoning?.default ? <IconCheck className='h-4 w-4 text-green-600' /> : '-'}
                                                </span>
                                              </div>
                                            </div>
                                          </div>
                                        </div>
                                      </div>
                                    </div>

                                    {/* Middle Section: Modalities + Limits (left) | Cost (right) */}
                                    <div className='grid grid-cols-1 gap-6 border-t pt-4 md:grid-cols-2'>
                                      {/* Left: Modalities + Limits */}
                                      <div className='space-y-4'>
                                        {/* Modalities */}
                                        <div className='space-y-3'>
                                          <h4 className='text-sm font-semibold'>{t('models.modelCard.modalities')}</h4>
                                          <div className='space-y-2 text-sm'>
                                            <div className='flex items-start gap-2'>
                                              <span className='text-muted-foreground shrink-0'>{t('models.modelCard.input')}:</span>
                                              <div className='flex flex-wrap gap-1'>
                                                {modelCard?.modalities?.input?.length
                                                  ? modelCard.modalities.input.map((m) => (
                                                      <Badge key={m} variant='outline' className='text-xs'>
                                                        {m}
                                                      </Badge>
                                                    ))
                                                  : '-'}
                                              </div>
                                            </div>
                                            <div className='flex items-start gap-2'>
                                              <span className='text-muted-foreground shrink-0'>{t('models.modelCard.output')}:</span>
                                              <div className='flex flex-wrap gap-1'>
                                                {modelCard?.modalities?.output?.length
                                                  ? modelCard.modalities.output.map((m) => (
                                                      <Badge key={m} variant='outline' className='text-xs'>
                                                        {m}
                                                      </Badge>
                                                    ))
                                                  : '-'}
                                              </div>
                                            </div>
                                          </div>
                                        </div>

                                        {/* Limits */}
                                        <div className='space-y-3'>
                                          <h4 className='text-sm font-semibold'>{t('models.modelCard.limit')}</h4>
                                          <div className='space-y-2 text-sm'>
                                            <div className='flex justify-between'>
                                              <span className='text-muted-foreground'>{t('models.modelCard.context')}:</span>
                                              <span className='font-mono text-xs'>{modelCard?.limit?.context?.toLocaleString() ?? '-'}</span>
                                            </div>
                                            <div className='flex justify-between'>
                                              <span className='text-muted-foreground'>{t('models.modelCard.output')}:</span>
                                              <span className='font-mono text-xs'>{modelCard?.limit?.output?.toLocaleString() ?? '-'}</span>
                                            </div>
                                          </div>
                                        </div>
                                      </div>

                                      {/* Right: Cost */}
                                      <div className='space-y-3'>
                                        <h4 className='text-sm font-semibold'>{t('models.modelCard.cost')} ($/M)</h4>
                                        <div className='space-y-2 text-sm'>
                                          <div className='flex justify-between'>
                                            <span className='text-muted-foreground'>{t('models.modelCard.input')}:</span>
                                            <span className='font-mono text-xs'>{modelCard?.cost?.input ?? '-'}</span>
                                          </div>
                                          <div className='flex justify-between'>
                                            <span className='text-muted-foreground'>{t('models.modelCard.output')}:</span>
                                            <span className='font-mono text-xs'>{modelCard?.cost?.output ?? '-'}</span>
                                          </div>
                                          {modelCard?.cost?.cacheRead !== undefined && (
                                            <div className='flex justify-between'>
                                              <span className='text-muted-foreground'>{t('models.modelCard.cacheRead')}:</span>
                                              <span className='font-mono text-xs'>{modelCard.cost.cacheRead}</span>
                                            </div>
                                          )}
                                          {modelCard?.cost?.cacheWrite !== undefined && (
                                            <div className='flex justify-between'>
                                              <span className='text-muted-foreground'>{t('models.modelCard.cacheWrite')}:</span>
                                              <span className='font-mono text-xs'>{modelCard.cost.cacheWrite}</span>
                                            </div>
                                          )}
                                        </div>
                                      </div>
                                    </div>

                                    {/* Bottom Section: Dates */}
                                    <div className='border-t pt-4'>
                                      <h4 className='mb-3 text-sm font-semibold'>{t('models.modelCard.dates')}</h4>
                                      <div className='flex gap-6 text-sm'>
                                        <div className='flex gap-2'>
                                          <span className='text-muted-foreground'>{t('models.modelCard.knowledge')}:</span>
                                          <span>{modelCard?.knowledge || '-'}</span>
                                        </div>
                                        <div className='flex gap-2'>
                                          <span className='text-muted-foreground'>{t('models.modelCard.releaseDate')}:</span>
                                          <span>{modelCard?.releaseDate || '-'}</span>
                                        </div>
                                        <div className='flex gap-2'>
                                          <span className='text-muted-foreground'>{t('models.modelCard.lastUpdated')}:</span>
                                          <span>{modelCard?.lastUpdated || '-'}</span>
                                        </div>
                                      </div>
                                    </div>
                                  </div>
                                </motion.div>
                            </TableCell>
                          </TableRow>
                              )}
                            </AnimatePresence>
                          </React.Fragment>
                        );
                      })}
                  </React.Fragment>
                );
              })
            ) : (
              <TableRow className='!bg-[var(--table-background)]'>
                <TableCell colSpan={columns.length} className='h-24 !bg-[var(--table-background)] text-center'>
                  {t('common.noData')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        </div>
      </div>

      {selectedCount > 0 && canWrite && (
        <div className='fixed bottom-6 left-1/2 z-50 -translate-x-1/2'>
          <div className='flex items-center gap-2 rounded-lg border bg-[var(--table-background)] px-4 py-2 shadow-lg'>
            <div className='bg-border mx-2 h-6 w-px' />
            <Button variant='ghost' size='icon' className='h-8 w-8' onClick={() => setRowSelection({})}>
              <IconX className='h-4 w-4' />
            </Button>
            <div className='flex items-center gap-1.5 px-2'>
              <span className='bg-primary text-primary-foreground flex h-6 min-w-6 items-center justify-center rounded px-1.5 text-xs font-medium'>
                {selectedCount}
              </span>
              <span className='text-muted-foreground text-sm'>{t('common.selected')}</span>
            </div>
            <div className='bg-border mx-2 h-6 w-px' />
            <PermissionGuard requiredScope='write_channels'>
              <>
                <Button
                  variant='ghost'
                  size='icon'
                  className='h-8 w-8 text-green-600 hover:bg-green-100 hover:text-green-700'
                  onClick={() => setOpen('bulkEnable')}
                  title={t('common.buttons.enable')}
                >
                  <IconCheck className='h-4 w-4' />
                </Button>
                <Button
                  variant='ghost'
                  size='icon'
                  className='h-8 w-8 text-amber-600 hover:bg-amber-100 hover:text-amber-700'
                  onClick={() => setOpen('bulkDisable')}
                  title={t('common.buttons.disable')}
                >
                  <IconBan className='h-4 w-4' />
                </Button>
                <Button
                  variant='ghost'
                  size='icon'
                  className='text-destructive h-8 w-8 hover:bg-red-100 hover:text-red-700'
                  onClick={() => setOpen('delete')}
                  title={t('common.buttons.delete')}
                >
                  <IconTrash className='h-4 w-4' />
                </Button>
              </>
            </PermissionGuard>
          </div>
        </div>
      )}
    </div>
  );
}
