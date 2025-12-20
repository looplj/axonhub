import { useCallback, useState } from 'react'
import { format } from 'date-fns'
import { ColumnDef, Row } from '@tanstack/react-table'
import { IconCheck, IconX, IconLink, IconChevronDown, IconChevronRight } from '@tabler/icons-react'
import { toc } from '@lobehub/icons'
import * as Icons from '@lobehub/icons'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useModels } from '../context/models-context'
import { Model } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'
import { ModelsStatusDialog } from './models-status-dialog'

// Status Switch Cell Component to handle status toggle with confirmation dialog
function StatusSwitchCell({ row }: { row: Row<Model> }) {
  const model = row.original
  const [dialogOpen, setDialogOpen] = useState(false)

  const isEnabled = model.status === 'enabled'
  const isArchived = model.status === 'archived'

  const handleSwitchClick = useCallback(() => {
    if (!isArchived) {
      setDialogOpen(true)
    }
  }, [isArchived])

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} disabled={isArchived} data-testid='model-status-switch' />
      {dialogOpen && <ModelsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={model} />}
    </>
  )
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t']): ColumnDef<Model>[] => {
  return [
    {
      id: 'expand',
      header: () => null,
      meta: {
        className: 'w-8 min-w-8',
      },
      cell: ({ row }) => (
        <Button variant='ghost' size='sm' className='h-6 w-6 p-0' onClick={() => row.toggleExpanded()}>
          {row.getIsExpanded() ? <IconChevronDown className='h-4 w-4' /> : <IconChevronRight className='h-4 w-4' />}
        </Button>
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('models.columns.selectAll')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('models.columns.selectRow')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'icon',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.icon')} />,
      cell: ({ row }) => {
        const model = row.original
        const iconName = model.icon
        const IconComponent = iconName && Icons[iconName as keyof typeof Icons]

        return (
          <div className='flex items-center justify-center'>
            {IconComponent ? (
              //@ts-ignore
              <IconComponent className='h-5 w-5' />
            ) : (
              <span className='text-muted-foreground text-xs'>-</span>
            )}
          </div>
        )
      },
      enableSorting: false,
      meta: {
        className: 'w-16',
      },
    },
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.name')} />,
      cell: ({ row }) => {
        const model = row.original
        return (
          <div className='flex max-w-56 items-center gap-2'>
            <div className='truncate font-medium'>{model.name}</div>
          </div>
        )
      },
      meta: {
        className: 'md:table-cell min-w-48',
      },
      enableHiding: false,
      enableSorting: true,
    },
    {
      accessorKey: 'modelID',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.modelId')} />,
      cell: ({ row }) => {
        return <span className='font-mono text-sm'>{row.getValue('modelID')}</span>
      },
      meta: {
        className: 'min-w-48',
      },
      enableSorting: true,
    },
    {
      accessorKey: 'developer',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.developer')} />,
      cell: ({ row }) => {
        return <Badge variant='outline'>{row.getValue('developer')}</Badge>
      },
      enableSorting: false,
    },
    {
      accessorKey: 'type',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.type')} />,
      cell: ({ row }) => {
        const type = row.getValue('type') as string
        return <Badge variant='secondary'>{t(`models.types.${type}`)}</Badge>
      },
      enableSorting: false,
    },
    // {
    //   id: 'capabilities',
    //   header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.capabilities')} />,
    //   cell: ({ row }) => {
    //     const model = row.original
    //     const modalities = model.modelCard?.modalities

    //     if (!modalities) {
    //       return <span className='text-muted-foreground text-xs'>-</span>
    //     }

    //     return (
    //       <div className='flex flex-col gap-1 text-xs'>
    //         <div className='flex items-center gap-1'>
    //           <span className='text-muted-foreground'>{t('models.columns.input')}:</span>
    //           <div className='flex flex-wrap gap-1'>
    //             {modalities.input?.map((input) => (
    //               <Badge key={input} variant='outline' className='text-xs'>
    //                 {input}
    //               </Badge>
    //             ))}
    //           </div>
    //         </div>
    //         <div className='flex items-center gap-1'>
    //           <span className='text-muted-foreground'>{t('models.columns.output')}:</span>
    //           <div className='flex flex-wrap gap-1'>
    //             {modalities.output?.map((output) => (
    //               <Badge key={output} variant='outline' className='text-xs'>
    //                 {output}
    //               </Badge>
    //             ))}
    //           </div>
    //         </div>
    //       </div>
    //     )
    //   },
    //   enableSorting: false,
    // },
    {
      id: 'toolCall',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.toolCall')} />,
      cell: ({ row }) => {
        const model = row.original
        const toolCall = model.modelCard?.toolCall

        return (
          <div className='flex justify-center'>
            {toolCall ? <IconCheck className='h-4 w-4 text-green-600' /> : <IconX className='text-muted-foreground h-4 w-4' />}
          </div>
        )
      },
      enableSorting: false,
    },
    // {
    //   id: 'context',
    //   header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.context')} />,
    //   cell: ({ row }) => {
    //     const model = row.original
    //     const limit = model.modelCard?.limit

    //     if (!limit) {
    //       return <span className='text-muted-foreground text-xs'>-</span>
    //     }

    //     return (
    //       <Tooltip>
    //         <TooltipTrigger asChild>
    //           <div className='cursor-help text-xs'>
    //             <div>
    //               <span className='text-muted-foreground'>{t('models.columns.contextWindow')}: </span>
    //               <span className='font-medium'>{limit.context?.toLocaleString()}</span>
    //             </div>
    //             <div>
    //               <span className='text-muted-foreground'>{t('models.columns.maxOutput')}: </span>
    //               <span className='font-medium'>{limit.output?.toLocaleString()}</span>
    //             </div>
    //           </div>
    //         </TooltipTrigger>
    //         <TooltipContent>
    //           <div className='space-y-1'>
    //             <p>
    //               {t('models.columns.contextWindowFull')}: {limit.context?.toLocaleString()}
    //             </p>
    //             <p>
    //               {t('models.columns.maxOutputFull')}: {limit.output?.toLocaleString()}
    //             </p>
    //           </div>
    //         </TooltipContent>
    //       </Tooltip>
    //     )
    //   },
    //   enableSorting: false,
    // },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.status')} />,
      cell: StatusSwitchCell,
      enableSorting: false,
      enableHiding: false,
    },
    {
      id: 'association',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.association')} />,
      cell: ({ row }) => {
        const model = row.original
        const { setOpen, setCurrentRow } = useModels()

        const handleOpenAssociationDialog = useCallback(() => {
          setCurrentRow(model)
          setOpen('association')
        }, [model, setCurrentRow, setOpen])

        const associationCount = model.settings?.associations?.length || 0

        return (
          <Button size='sm' variant='outline' className='h-8 px-3' onClick={handleOpenAssociationDialog}>
            <IconLink className='mr-1 h-3 w-3' />
            {associationCount > 0 ? `${associationCount}` : t('models.actions.addAssociation')}
          </Button>
        )
      },
      enableSorting: false,
    },

    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('models.columns.createdAt')} />,
      cell: ({ row }) => {
        const raw = row.getValue('createdAt') as unknown
        const date = raw instanceof Date ? raw : new Date(raw as string)

        if (Number.isNaN(date.getTime())) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div className='text-muted-foreground cursor-help text-sm'>{format(date, 'yyyy-MM-dd')}</div>
            </TooltipTrigger>
            <TooltipContent>{format(date, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
          </Tooltip>
        )
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      id: 'actions',
      header: () => null,
      cell: DataTableRowActions,
      meta: {
        className: 'w-[56px] min-w-[56px] pr-3 pl-0',
      },
      enableSorting: false,
      enableHiding: false,
    },
  ]
}
