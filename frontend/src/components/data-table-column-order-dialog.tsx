'use client';

import { useState, useEffect, useMemo } from 'react';
import { DndContext, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core';
import { arrayMove, SortableContext, sortableKeyboardCoordinates, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { Table } from '@tanstack/react-table';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { getReorderableColumnIds, reconcileColumnOrder } from '@/lib/column-order';

interface SortableColumnRowProps {
  id: string;
  label: string;
}

function SortableColumnRow({ id, label }: SortableColumnRowProps) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div ref={setNodeRef} style={style} className='flex items-center gap-3 rounded-lg border bg-background p-3'>
      <button
        {...attributes}
        {...listeners}
        className='cursor-grab text-muted-foreground hover:text-foreground active:cursor-grabbing'
        aria-label={t('common.dragToReorder', { label })}
      >
        <GripVertical className='h-5 w-5' />
      </button>
      <span className='flex-1 truncate text-sm font-medium'>{label}</span>
    </div>
  );
}

interface DataTableColumnOrderDialogProps<TData> {
  table: Table<TData>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  getColumnLabel?: (columnId: string) => string;
}

export function DataTableColumnOrderDialog<TData>({
  table,
  open,
  onOpenChange,
  getColumnLabel,
}: DataTableColumnOrderDialogProps<TData>) {
  const { t } = useTranslation();
  const reorderableIds = useMemo(() => getReorderableColumnIds(table), [table, open]);

  const [items, setItems] = useState<string[]>([]);

  useEffect(() => {
    if (open) {
      const current = table.getState().columnOrder;
      const seeded = current.length > 0 ? current.filter((id) => reorderableIds.includes(id)) : [...reorderableIds];
      const missing = reorderableIds.filter((id) => !seeded.includes(id));
      setItems([...seeded, ...missing]);
    }
  }, [open, reorderableIds, table.getState().columnOrder]);

  const labelOf = (id: string): string => {
    if (getColumnLabel) return getColumnLabel(id);
    return t(`common.columns.${id}`, { defaultValue: id });
  };

  const applyOrder = (nextReorderable: string[]) => {
    const allIds = table.getAllLeafColumns().map((c) => c.id);
    table.setColumnOrder(
      reconcileColumnOrder({
        allColumnIds: allIds,
        reorderableColumnIds: reorderableIds,
        storedOrder: nextReorderable,
        pinnedLast: ['details'],
        pinned: ['status', 'source'],
      }),
    );
  };

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (over && active.id !== over.id) {
      const oldIndex = items.findIndex((id) => id === active.id);
      const newIndex = items.findIndex((id) => id === over.id);
      const next = arrayMove(items, oldIndex, newIndex);
      setItems(next);
      applyOrder(next);
    }
  };

  const handleReset = () => {
    table.setColumnOrder([]);
    setItems(getReorderableColumnIds(table));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[80vh] max-w-[24rem]'>
        <DialogHeader>
          <DialogTitle>{t('common.columnOrderDialogTitle')}</DialogTitle>
          <DialogDescription>{t('common.columnOrderDialogDescription')}</DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-y-auto py-2'>
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <SortableContext items={items} strategy={verticalListSortingStrategy}>
              <div className='flex flex-col gap-2'>
                {items.map((id) => (
                  <SortableColumnRow key={id} id={id} label={labelOf(id)} />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleReset}>
            {t('common.resetColumnOrder')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
