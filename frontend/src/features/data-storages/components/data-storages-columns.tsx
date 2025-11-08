import { ColumnDef } from '@tanstack/react-table'
import { DataStorage } from '../data/data-storages'
import { Badge } from '@/components/ui/badge'
import { DataStorageActions } from './data-storage-actions'
import { TFunction } from 'i18next'

export const createColumns = (t: TFunction, defaultDataStorageID?: string | null): ColumnDef<DataStorage>[] => [
  {
    accessorKey: 'name',
    header: t('dataStorages.columns.name'),
    cell: ({ row }) => {
      const isDefault = defaultDataStorageID === row.original.id

      return (
        <div className='flex items-center gap-2'>
          <span className='font-medium'>{row.getValue('name')}</span>
          {isDefault && (
            <Badge variant='outline' className='text-xs font-normal'>
              {t('dataStorages.default')}
            </Badge>
          )}
        </div>
      )
    },
  },
  {
    accessorKey: 'primary',
    header: t('dataStorages.columns.primary'),
    cell: ({ row }) => {
      const isPrimary = row.original.primary
      return isPrimary ? (
        <Badge variant='secondary' className='text-xs'>
          {t('dataStorages.primary')}
        </Badge>
      ) : (
        <span className='text-muted-foreground'>-</span>
      )
    },
  },
  {
    accessorKey: 'description',
    header: t('dataStorages.columns.description'),
    cell: ({ row }) => {
      const description = row.getValue('description') as string
      return <span className='text-muted-foreground'>{description || '-'}</span>
    },
  },
  {
    accessorKey: 'type',
    header: t('dataStorages.columns.type'),
    cell: ({ row }) => {
      const type = row.getValue('type') as string
      const typeLabels: Record<string, string> = {
        database: t('dataStorages.types.database'),
        fs: t('dataStorages.types.fs'),
        s3: t('dataStorages.types.s3'),
        gcs: t('dataStorages.types.gcs'),
      }
      return <Badge variant='outline'>{typeLabels[type] || type}</Badge>
    },
  },
  {
    accessorKey: 'settings',
    header: t('dataStorages.columns.settings'),
    cell: ({ row }) => {
      const settings = row.getValue('settings') as DataStorage['settings']
      const type = row.original.type
      
      if (type === 'fs' && settings.directory) {
        return (
          <span className='font-mono text-sm text-muted-foreground'>
            {settings.directory}
          </span>
        )
      }

      if (type === 's3' && settings.s3?.bucketName) {
        return (
          <span className='font-mono text-sm text-muted-foreground'>
            {settings.s3.bucketName}
          </span>
        )
      }

      if (type === 'gcs' && settings.gcs?.bucketName) {
        return (
          <span className='font-mono text-sm text-muted-foreground'>
            {settings.gcs.bucketName}
          </span>
        )
      }
      
      return <span className='text-muted-foreground'>-</span>
    },
  },
  {
    accessorKey: 'status',
    header: t('dataStorages.columns.status'),
    cell: ({ row }) => {
      const status = row.getValue('status') as string
      const statusVariants: Record<string, 'default' | 'secondary'> = {
        active: 'default',
        archived: 'secondary',
      }
      const statusLabels: Record<string, string> = {
        active: t('dataStorages.status.active'),
        archived: t('dataStorages.status.archived'),
      }
      return (
        <Badge variant={statusVariants[status] || 'default'}>
          {statusLabels[status] || status}
        </Badge>
      )
    },
  },
  {
    id: 'actions',
    header: t('dataStorages.columns.actions'),
    cell: ({ row }) => (
      <DataStorageActions dataStorage={row.original} defaultDataStorageID={defaultDataStorageID} />
    ),
  },
]
