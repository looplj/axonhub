import { useState, useEffect, memo, useCallback } from 'react';
import { DndContext, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core';
import { arrayMove, SortableContext, sortableKeyboardCoordinates, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, ArrowUpToLine, ArrowDownToLine } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { useAllChannelsForOrdering, useBulkUpdateChannelOrdering } from '../data/channels';
import { ChannelOrderingItem } from '../data/schema';

const WEIGHT_PRECISION = 0;
const MIN_WEIGHT = 0;
const MAX_WEIGHT = 100;

const formatWeight = (value: number) => Math.round(value);

const clampWeight = (value: number) => formatWeight(Math.min(MAX_WEIGHT, Math.max(MIN_WEIGHT, value)));

const calculateRelativeWeight = (prev?: number, next?: number) => {
  if (prev == null && next == null) {
    return clampWeight(1);
  }
  if (prev == null) {
    return clampWeight((next ?? 0) + 1);
  }
  if (next == null) {
    return clampWeight(prev - 1);
  }
  if (prev === next) {
    return clampWeight(prev);
  }
  return clampWeight(Math.floor((prev + next) / 2));
};

interface ChannelOrderingItemProps {
  channel: ChannelOrderingItem;
  orderingWeight: number;
  index: number;
  total: number;
  onMoveToTop: (index: number) => void;
  onMoveToBottom: (index: number) => void;
  onWeightChange: (id: string, weight: number) => void;
}

const ChannelOrderingItemComponent = memo(function ChannelOrderingItemComponent({
  channel,
  orderingWeight,
  index,
  total,
  onMoveToTop,
  onMoveToBottom,
  onWeightChange,
}: ChannelOrderingItemProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: channel.id });
  const { t } = useTranslation();
  const [localWeight, setLocalWeight] = useState(orderingWeight.toString());

  useEffect(() => {
    setLocalWeight(orderingWeight.toString());
  }, [orderingWeight]);

  const handleWeightBlur = () => {
    if (localWeight.trim() === '') {
      setLocalWeight(orderingWeight.toString());
      return;
    }

    const val = Number(localWeight);
    if (!Number.isNaN(val) && val !== orderingWeight) {
      onWeightChange(channel.id, val);
    } else {
      setLocalWeight(orderingWeight.toString());
    }
  };

  const getTypeDisplayName = (type: string) => {
    const typeKey = `channels.types.${type}` as const;
    return t(typeKey, type);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'enabled':
        return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-400 dark:border-emerald-800';
      case 'disabled':
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700';
      case 'archived':
        return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:border-amber-800';
      default:
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700';
    }
  };

  const getTypeColor = (type: string) => {
    const colors = {
      openai: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-400',
      anthropic: 'bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950 dark:text-purple-400',
      deepseek: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-950 dark:text-indigo-400',
      doubao: 'bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-950 dark:text-orange-400',
      kimi: 'bg-pink-50 text-pink-700 border-pink-200 dark:bg-pink-950 dark:text-pink-400',
    };
    return colors[type as keyof typeof colors] || 'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-900 dark:text-gray-400';
  };

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`group bg-card flex items-center gap-2 rounded-md border p-1 hover:shadow-sm ${
        isDragging ? 'ring-primary/20 relative z-50 shadow-xl ring-2' : 'hover:border-primary/20'
      }`}
    >
      {/* Drag Handle */}
      <div
        className='text-muted-foreground hover:text-foreground flex min-w-[40px] cursor-grab items-center gap-1 px-1 active:cursor-grabbing'
        {...attributes}
        {...listeners}
      >
        <GripVertical className='h-3.5 w-3.5' />
        <span className='w-[20px] text-center font-mono text-[10px]'>{index + 1}</span>
      </div>

      {/* Channel Info - Single Line Optimized */}
      <div className='flex min-w-0 flex-1 items-center gap-2'>
        <div className='flex min-w-0 items-center gap-1.5'>
          <span className='truncate text-sm font-medium'>{channel.name}</span>
          <div className='flex flex-shrink-0 gap-1'>
            <Badge variant='outline' className={`h-3.5 px-1 text-[10px] font-normal ${getTypeColor(channel.type)}`}>
              {getTypeDisplayName(channel.type)}
            </Badge>
            <Badge variant='outline' className={`h-3.5 px-1 text-[10px] font-normal ${getStatusColor(channel.status)}`}>
              {t(`channels.status.${channel.status}`)}
            </Badge>
          </div>
        </div>

        <div className='hidden flex-1 items-center gap-2 sm:flex'>
          <div className='bg-border h-3 w-[1px]' />
          <span className='text-muted-foreground truncate font-mono text-[10px] opacity-70'>{channel.baseURL}</span>
        </div>
      </div>

      {/* Controls */}
      <div className='flex items-center gap-1 pr-1'>
        <div className='bg-muted/30 hidden items-center gap-1.5 rounded px-1.5 py-0.5 sm:flex'>
          <span className='text-muted-foreground text-[10px]'>{t('channels.dialogs.bulkOrdering.orderingWeight')}</span>
          <Input
            type='number'
            inputMode='decimal'
            step='any'
            min={MIN_WEIGHT}
            max={MAX_WEIGHT}
            className='h-6 w-16 px-1 text-center text-xs'
            value={localWeight}
            onChange={(e) => setLocalWeight(e.target.value)}
            onBlur={handleWeightBlur}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.currentTarget.blur();
              }
            }}
            onClick={(e) => e.stopPropagation()}
            onPointerDown={(e) => e.stopPropagation()}
          />
        </div>

        <div className='flex items-center gap-0.5'>
          <Button
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground h-6 w-6'
            onClick={() => onMoveToTop(index)}
            disabled={index === 0}
            title={t('common.moveToTop', 'Move to top')}
          >
            <ArrowUpToLine className='h-3.5 w-3.5' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground h-6 w-6'
            onClick={() => onMoveToBottom(index)}
            disabled={index === total - 1}
            title={t('common.moveToBottom', 'Move to bottom')}
          >
            <ArrowDownToLine className='h-3.5 w-3.5' />
          </Button>
        </div>
      </div>
    </div>
  );
});

interface ChannelsBulkOrderingDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsBulkOrderingDialog({ open, onOpenChange }: ChannelsBulkOrderingDialogProps) {
  const { t } = useTranslation();

  // Only fetch channels when dialog is open (lazy loading)
  const { data: channelsData, isLoading } = useAllChannelsForOrdering({
    enabled: open, // Only fetch when dialog is open
  });

  const bulkUpdateMutation = useBulkUpdateChannelOrdering();

  // Local state for ordering
  const [orderedChannels, setOrderedChannels] = useState<Array<{ channel: ChannelOrderingItem; orderingWeight: number }>>([]);
  const [hasChanges, setHasChanges] = useState(false);

  // Initialize ordered channels when data loads
  useEffect(() => {
    if (channelsData?.edges) {
      const channels = channelsData.edges.map((edge, index) => ({
        channel: edge.node,
        orderingWeight: clampWeight(edge.node.orderingWeight ?? channelsData.edges.length - index),
      }));
      // Sort by orderingWeight DESC (higher weight first)
      channels.sort((a, b) => b.orderingWeight - a.orderingWeight);
      setOrderedChannels(channels);
      setHasChanges(false);
    }
  }, [channelsData]);

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;

    if (!over || active.id === over.id) {
      return;
    }

    setOrderedChannels((items) => {
      const oldIndex = items.findIndex((item) => item.channel.id === active.id);
      const newIndex = items.findIndex((item) => item.channel.id === over.id);

      if (oldIndex === -1 || newIndex === -1) {
        return items;
      }

      const newItems = arrayMove(items, oldIndex, newIndex);
      const prevWeight = newItems[newIndex - 1]?.orderingWeight;
      const nextWeight = newItems[newIndex + 1]?.orderingWeight;

      newItems[newIndex] = {
        ...newItems[newIndex],
        orderingWeight: calculateRelativeWeight(prevWeight, nextWeight),
      };

      setHasChanges(true);
      return newItems;
    });
  }, []);

  const handleWeightChange = useCallback((id: string, weight: number) => {
    const normalizedWeight = clampWeight(weight);
    setOrderedChannels((items) => {
      const newItems = items.map((item) => (item.channel.id === id ? { ...item, orderingWeight: normalizedWeight } : item));
      // Sort by orderingWeight DESC (higher weight first)
      // Maintain stable sort for equal weights? Javascript sort is stable.
      newItems.sort((a, b) => b.orderingWeight - a.orderingWeight);
      setHasChanges(true);
      return newItems;
    });
  }, []);

  const handleMoveToTop = useCallback((index: number) => {
    setOrderedChannels((items) => {
      if (!items.length || index === 0) {
        return items;
      }

      const newItems = arrayMove(items, index, 0);
      const nextWeight = newItems[1]?.orderingWeight;

      newItems[0] = {
        ...newItems[0],
        orderingWeight: calculateRelativeWeight(undefined, nextWeight),
      };

      setHasChanges(true);
      return newItems;
    });
  }, []);

  const handleMoveToBottom = useCallback((index: number) => {
    setOrderedChannels((items) => {
      if (!items.length || index === items.length - 1) {
        return items;
      }

      const targetIndex = items.length - 1;
      const newItems = arrayMove(items, index, targetIndex);
      const prevWeight = newItems[targetIndex - 1]?.orderingWeight;

      newItems[targetIndex] = {
        ...newItems[targetIndex],
        orderingWeight: calculateRelativeWeight(prevWeight, undefined),
      };

      setHasChanges(true);
      return newItems;
    });
  }, []);

  const handleSave = async () => {
    try {
      const updates = orderedChannels.map((item) => ({
        id: item.channel.id,
        orderingWeight: item.orderingWeight,
      }));

      await bulkUpdateMutation.mutateAsync({
        channels: updates,
      });

      setHasChanges(false);
      onOpenChange(false);
    } catch (_error) {
      // Error is handled by the mutation hook
    }
  };

  const handleCancel = () => {
    // Reset to original order
    if (channelsData?.edges) {
      const channels = channelsData.edges.map((edge, index) => ({
        channel: edge.node,
        orderingWeight: clampWeight(edge.node.orderingWeight ?? channelsData.edges.length - index),
      }));
      channels.sort((a, b) => b.orderingWeight - a.orderingWeight);
      setOrderedChannels(channels);
      setHasChanges(false);
    }
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-5xl'>
        <DialogHeader className='flex-shrink-0 text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <GripVertical className='text-muted-foreground h-5 w-5' />
            {t('channels.dialogs.bulkOrdering.title')}
          </DialogTitle>
          <DialogDescription className='text-muted-foreground text-sm'>{t('channels.dialogs.bulkOrdering.description')}</DialogDescription>
        </DialogHeader>

        <Separator className='flex-shrink-0' />

        <div className='-mr-4 h-[40rem] w-full flex-1 overflow-y-auto py-1 pr-4'>
          {isLoading ? (
            <div className='flex items-center justify-center py-12'>
              <div className='flex flex-col items-center gap-3'>
                <div className='border-primary h-8 w-8 animate-spin rounded-full border-b-2'></div>
                <div className='text-muted-foreground text-sm'>{t('common.loading', 'Loading')}...</div>
              </div>
            </div>
          ) : orderedChannels.length === 0 ? (
            <div className='flex items-center justify-center py-12'>
              <div className='flex flex-col items-center gap-3 text-center'>
                <GripVertical className='text-muted-foreground/30 h-12 w-12' />
                <div className='text-muted-foreground text-sm'>{t('channels.dialogs.bulkOrdering.noChannels')}</div>
              </div>
            </div>
          ) : (
            <div className='flex h-full flex-col gap-4 p-0.5'>
              {/* Summary Header */}
              <div className='flex items-center justify-between px-1 py-2'>
                <div className='text-muted-foreground flex items-center gap-4 text-sm'>
                  <span>{t('channels.dialogs.bulkOrdering.dragHint')}</span>
                  <Badge variant='secondary' className='font-mono'>
                    {t('channels.dialogs.bulkOrdering.channelCount', {
                      count: orderedChannels.length,
                    })}
                  </Badge>
                  {hasChanges && (
                    <Badge
                      variant='outline'
                      className='border-amber-200 bg-amber-50 text-amber-600 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-400'
                    >
                      {t('common.unsavedChanges', 'Unsaved changes')}
                    </Badge>
                  )}
                </div>
              </div>

              {/* Channels List */}
              <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                <SortableContext items={orderedChannels.map((item) => item.channel.id)} strategy={verticalListSortingStrategy}>
                  <div className='flex-1 space-y-1'>
                    {orderedChannels.map((item, index) => (
                      <ChannelOrderingItemComponent
                        key={item.channel.id}
                        channel={item.channel}
                        orderingWeight={item.orderingWeight}
                        index={index}
                        total={orderedChannels.length}
                        onMoveToTop={handleMoveToTop}
                        onMoveToBottom={handleMoveToBottom}
                        onWeightChange={handleWeightChange}
                      />
                    ))}
                  </div>
                </SortableContext>
              </DndContext>
            </div>
          )}
        </div>

        <DialogFooter className='flex-shrink-0'>
          <div className='flex w-full items-center justify-between'>
            <div className='text-muted-foreground text-xs'>
              {hasChanges && (
                <span className='flex items-center gap-1'>
                  <div className='h-2 w-2 rounded-full bg-amber-500'></div>
                  {t('common.unsavedChanges', 'You have unsaved changes')}
                </span>
              )}
            </div>
            <div className='flex items-center gap-2'>
              <Button variant='outline' onClick={handleCancel}>
                {t('common.buttons.cancel')}
              </Button>
              <Button onClick={handleSave} disabled={!hasChanges || bulkUpdateMutation.isPending} className='min-w-[120px]'>
                {bulkUpdateMutation.isPending ? (
                  <div className='flex items-center gap-2'>
                    <div className='h-4 w-4 animate-spin rounded-full border-b-2 border-white'></div>
                    {t('common.buttons.saving')}
                  </div>
                ) : (
                  t('channels.dialogs.bulkOrdering.saveButton')
                )}
              </Button>
            </div>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
