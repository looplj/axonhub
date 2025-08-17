import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { Row } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { IconEdit, IconUserOff, IconUserCheck, IconKey } from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useUsers } from '../context/users-context'
import { User } from '../data/schema'

interface DataTableRowActionsProps {
  row: Row<User>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useUsers()
  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button
            variant='ghost'
            className='data-[state=open]:bg-muted flex h-8 w-8 p-0'
          >
            <DotsHorizontalIcon className='h-4 w-4' />
            <span className='sr-only'>{t('users.actions.openMenu')}</span>
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
            {t('users.actions.edit')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('changePassword')
            }}
          >
            <IconKey size={16} className="mr-2" />
            {t('users.actions.changePassword')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('status')
            }}
            className={row.original.status === 'activated' ? 'text-red-500!' : 'text-green-500!'}
          >
            {row.original.status === 'activated' ? <IconUserOff size={16} className="mr-2" /> : <IconUserCheck size={16} className="mr-2" />}
            {row.original.status === 'activated' ? t('users.actions.deactivate') : t('users.actions.activate')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  )
}
