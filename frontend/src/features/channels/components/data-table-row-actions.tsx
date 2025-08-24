import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { Row } from '@tanstack/react-table'
import { IconEdit, IconToggleLeft, IconToggleRight, IconSettings, IconArchive } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
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
  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button
            variant='ghost'
            className='data-[state=open]:bg-muted flex h-8 w-8 p-0'
          >
            <DotsHorizontalIcon className='h-4 w-4' />
            <span className='sr-only'>{t('channels.actions.openMenu')}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[160px]'>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('edit')
            }}
          >
            <IconEdit size={16} className="mr-2" />
            {t('channels.actions.edit')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('settings')
            }}
          >
            <IconSettings size={16} className="mr-2" />
            {t('channels.actions.settings')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('status')
            }}
            className={row.original.status === 'enabled' ? 'text-red-500!' : 'text-green-500!'}
          >
            {row.original.status === 'enabled' ? <IconToggleLeft size={16} className="mr-2" /> : <IconToggleRight size={16} className="mr-2" />}
            {row.original.status === 'enabled' ? t('channels.actions.disable') : t('channels.actions.enable')}
          </DropdownMenuItem>
          {row.original.status !== 'archived' && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(row.original)
                setOpen('archive')
              }}
              className="text-orange-500!"
            >
              <IconArchive size={16} className="mr-2" />
              {t('channels.actions.archive')}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  )
}