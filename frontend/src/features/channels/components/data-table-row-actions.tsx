import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { Row } from '@tanstack/react-table'
import {
  IconEdit,
  IconCopy,
  IconToggleLeft,
  IconToggleRight,
  IconArchive,
  IconRoute,
  IconAdjustments,
  IconTrash,
  IconNetwork,
  IconCheck,
  IconWeight,
} from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { usePermissions } from '@/hooks/usePermissions'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useChannels } from '../context/channels-context'
import { Channel } from '../data/schema'

interface DataTableRowActionsProps {
  row: Row<Channel>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useChannels()
  const { channelPermissions } = usePermissions()
  const channel = row.original
  const hasError = !!channel.errorMessage

  // Don't show menu if user has no permissions
  if (!channelPermissions.canWrite) {
    return null
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0' data-testid='row-actions'>
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('common.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        {/* Edit - requires write permission */}
        {channelPermissions.canEdit && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('edit')
            }}
          >
            <IconEdit size={16} className='mr-2' />
            {t('common.actions.edit')}
          </DropdownMenuItem>
        )}

        {channelPermissions.canEdit && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('duplicate')
            }}
          >
            <IconCopy size={16} className='mr-2' />
            {t('common.actions.duplicate')}
          </DropdownMenuItem>
        )}

        {/* Model Mapping - requires write permission */}
        {channelPermissions.canWrite && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('modelMapping')
            }}
          >
            <IconRoute size={16} className='mr-2' />
            {t('channels.dialogs.settings.modelMapping.title')}
          </DropdownMenuItem>
        )}

        {/* Overrides - requires write permission */}
        {channelPermissions.canWrite && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('overrides')
            }}
          >
            <IconAdjustments size={16} className='mr-2' />
            {t('channels.dialogs.settings.overrides.action')}
          </DropdownMenuItem>
        )}

        {/* Proxy - requires write permission */}
        {channelPermissions.canWrite && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('proxy')
            }}
          >
            <IconNetwork size={16} className='mr-2' />
            {t('channels.dialogs.proxy.action')}
          </DropdownMenuItem>
        )}

        {/* Weight - requires write permission */}
        {channelPermissions.canWrite && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('weight')
            }}
          >
            <IconWeight size={16} className='mr-2' />
            {t('channels.dialogs.weight.action')}
          </DropdownMenuItem>
        )}

        {/* Error Resolved - requires write permission and error message */}
        {channelPermissions.canWrite && hasError && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel)
              setOpen('errorResolved')
            }}
            className='text-green-500!'
          >
            <IconCheck size={16} className='mr-2' />
            {t('channels.actions.errorResolved')}
          </DropdownMenuItem>
        )}

        {/* Separator only if there are both read and write actions */}
        {channelPermissions.canRead && channelPermissions.canWrite && <DropdownMenuSeparator />}

        {/* Status toggle - requires write permission */}
        {channelPermissions.canWrite && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('status')
            }}
            className={row.original.status === 'enabled' ? 'text-red-500!' : 'text-green-500!'}
          >
            {row.original.status === 'enabled' ? (
              <IconToggleLeft size={16} className='mr-2' />
            ) : (
              <IconToggleRight size={16} className='mr-2' />
            )}
            {row.original.status === 'enabled' ? t('common.buttons.disable') : t('common.buttons.enable')}
          </DropdownMenuItem>
        )}

        {/* Archive - requires write permission */}
        {channelPermissions.canWrite && row.original.status !== 'archived' && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('archive')
            }}
            className='text-orange-500!'
          >
            <IconArchive size={16} className='mr-2' />
            {t('common.buttons.archive')}
          </DropdownMenuItem>
        )}

        {/* Delete - requires write permission */}
        {channelPermissions.canWrite && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('delete')
            }}
            className='text-red-500!'
          >
            <IconTrash size={16} className='mr-2' />
            {t('common.buttons.delete')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
