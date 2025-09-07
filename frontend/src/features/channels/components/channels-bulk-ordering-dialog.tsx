import { useState, useEffect } from 'react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { arrayMove, SortableContext, sortableKeyboardCoordinates, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { GripVertical } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { useAllChannelsForOrdering, useBulkUpdateChannelOrdering } from '../data/channels'
import { ChannelOrderingItem } from '../data/schema'

interface ChannelOrderingItemProps {
  channel: ChannelOrderingItem
  orderingWeight: number
  index: number
  total: number
}

function ChannelOrderingItemComponent({ channel, orderingWeight, index, total }: ChannelOrderingItemProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: channel.id })
  const { t } = useTranslation()

  const getTypeDisplayName = (type: string) => {
    const typeKey = `channels.types.${type}` as const
    return t(typeKey, type)
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'enabled':
        return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-400 dark:border-emerald-800'
      case 'disabled':
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700'
      case 'archived':
        return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:border-amber-800'
      default:
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700'
    }
  }

  const getTypeColor = (type: string) => {
    const colors = {
      openai: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-400',
      anthropic: 'bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950 dark:text-purple-400',
      deepseek: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-950 dark:text-indigo-400',
      doubao: 'bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-950 dark:text-orange-400',
      kimi: 'bg-pink-50 text-pink-700 border-pink-200 dark:bg-pink-950 dark:text-pink-400',
    }
    return (
      colors[type as keyof typeof colors] ||
      'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-900 dark:text-gray-400'
    )
  }

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  return (
    <Card
      ref={setNodeRef}
      style={style}
      className={`group border-muted/50 hover:border-muted bg-card transition-all duration-200 hover:shadow-md ${
        isDragging ? 'ring-primary/20 shadow-lg ring-2' : ''
      }`}
    >
      <CardContent className='p-1.5'>
        <div className='flex items-center gap-2'>
          {/* Drag Handle */}
          <div
            className='flex min-w-[70px] cursor-grab items-center gap-2 active:cursor-grabbing'
            {...attributes}
            {...listeners}
          >
            <GripVertical
              className={`text-muted-foreground h-3 w-3 transition-all ${
                isDragging ? 'text-primary' : 'opacity-40 group-hover:opacity-70'
              }`}
            />
            <div className='text-muted-foreground min-w-[40px] text-center font-mono text-xs whitespace-nowrap'>
              {index + 1}/{total}
            </div>
          </div>

          {/* Channel Info */}
          <div className='min-w-0 flex-1'>
            <div className='flex items-center gap-1.5'>
              <h3 className='text-foreground truncate text-xs leading-tight font-medium'>{channel.name}</h3>
              <Badge variant='outline' className={`h-4 border px-1 text-xs ${getTypeColor(channel.type)}`}>
                {getTypeDisplayName(channel.type)}
              </Badge>
              <Badge variant='outline' className={`h-4 border px-1 text-xs ${getStatusColor(channel.status)}`}>
                {t(`channels.status.${channel.status}`)}
              </Badge>
            </div>
            <div className='text-muted-foreground bg-muted/20 truncate rounded px-1 py-0.5 font-mono text-xs leading-tight'>
              {channel.baseURL}
            </div>
          </div>

          {/* Priority and Controls */}
          <div className='flex items-center gap-1'>
            {/* Priority Display */}
            <div className='flex flex-col items-center'>
              <div className='text-muted-foreground text-xs leading-tight font-medium'>
                {t('channels.dialogs.bulkOrdering.orderingWeight')}
              </div>
              <div className='bg-primary/10 text-primary rounded border px-1.5 py-0.5 font-mono text-xs font-bold'>
                {orderingWeight}
              </div>
            </div>

            {/* Drag Status Indicator */}
            {isDragging && (
              <div className='flex flex-col items-center'>
                <div className='text-primary text-xs leading-tight font-medium'>{t('common.dragging', 'Dragging')}</div>
                <div className='bg-primary h-1.5 w-1.5 animate-pulse rounded-full'></div>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

interface ChannelsBulkOrderingDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ChannelsBulkOrderingDialog({ open, onOpenChange }: ChannelsBulkOrderingDialogProps) {
  const { t } = useTranslation()

  // Only fetch channels when dialog is open (lazy loading)
  const { data: channelsData, isLoading } = useAllChannelsForOrdering({
    enabled: open, // Only fetch when dialog is open
  })

  const bulkUpdateMutation = useBulkUpdateChannelOrdering()

  // Local state for ordering
  const [orderedChannels, setOrderedChannels] = useState<
    Array<{ channel: ChannelOrderingItem; orderingWeight: number }>
  >([])
  const [hasChanges, setHasChanges] = useState(false)

  // Initialize ordered channels when data loads
  useEffect(() => {
    if (channelsData?.edges) {
      const channels = channelsData.edges.map((edge, index) => ({
        channel: edge.node,
        orderingWeight: edge.node.orderingWeight || channelsData.edges.length - index,
      }))
      // Sort by orderingWeight DESC (higher weight first)
      channels.sort((a, b) => b.orderingWeight - a.orderingWeight)
      setOrderedChannels(channels)
      setHasChanges(false)
    }
  }, [channelsData])

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 10,
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event

    if (active.id !== over?.id) {
      setOrderedChannels((items) => {
        const oldIndex = items.findIndex((item) => item.channel.id === active.id)
        const newIndex = items.findIndex((item) => item.channel.id === over?.id)

        const newItems = arrayMove(items, oldIndex, newIndex)

        // Recalculate ordering weights based on new positions
        const updatedItems = newItems.map((item, idx) => ({
          ...item,
          orderingWeight: newItems.length - idx,
        }))

        setHasChanges(true)
        return updatedItems
      })
    }
  }

  const handleSave = async () => {
    try {
      const updates = orderedChannels.map((item) => ({
        id: item.channel.id,
        orderingWeight: item.orderingWeight,
      }))

      await bulkUpdateMutation.mutateAsync({
        channels: updates,
      })

      setHasChanges(false)
      onOpenChange(false)
    } catch (_error) {
      // Error is handled by the mutation hook
    }
  }

  const handleCancel = () => {
    // Reset to original order
    if (channelsData?.edges) {
      const channels = channelsData.edges.map((edge, index) => ({
        channel: edge.node,
        orderingWeight: edge.node.orderingWeight || channelsData.edges.length - index,
      }))
      channels.sort((a, b) => b.orderingWeight - a.orderingWeight)
      setOrderedChannels(channels)
      setHasChanges(false)
    }
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-5xl'>
        <DialogHeader className='flex-shrink-0 text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <GripVertical className='text-muted-foreground h-5 w-5' />
            {t('channels.dialogs.bulkOrdering.title')}
          </DialogTitle>
          <DialogDescription className='text-muted-foreground text-sm'>
            {t('channels.dialogs.bulkOrdering.description')}
          </DialogDescription>
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
                    {orderedChannels.length} {orderedChannels.length === 1 ? 'channel' : 'channels'}
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
                <SortableContext
                  items={orderedChannels.map((item) => item.channel.id)}
                  strategy={verticalListSortingStrategy}
                >
                  <div className='flex-1 space-y-1'>
                    {orderedChannels.map((item, index) => (
                      <ChannelOrderingItemComponent
                        key={item.channel.id}
                        channel={item.channel}
                        orderingWeight={item.orderingWeight}
                        index={index}
                        total={orderedChannels.length}
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
                {t('channels.dialogs.bulkOrdering.cancel')}
              </Button>
              <Button
                onClick={handleSave}
                disabled={!hasChanges || bulkUpdateMutation.isPending}
                className='min-w-[120px]'
              >
                {bulkUpdateMutation.isPending ? (
                  <div className='flex items-center gap-2'>
                    <div className='h-4 w-4 animate-spin rounded-full border-b-2 border-white'></div>
                    {t('channels.dialogs.bulkOrdering.saving')}
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
  )
}
