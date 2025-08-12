import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { IconUserOff, IconUserCheck } from '@tabler/icons-react'
import { Row } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ApiKey } from '../data/schema'
import { useApiKeysContext } from '../context/apikeys-context'

interface DataTableRowActionsProps {
  row: Row<ApiKey>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const { openDialog } = useApiKeysContext()
  const apiKey = row.original

  const handleEdit = (apiKey: ApiKey) => {
    openDialog('edit', apiKey)
  }

  const handleStatusChange = (apiKey: ApiKey) => {
    openDialog('status', apiKey)
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant='ghost'
          className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'
        >
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>Open menu</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
          <DropdownMenuItem onClick={() => handleEdit(apiKey)}>
            {t('apikeys.actions.edit')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => handleStatusChange(apiKey)}
            className={apiKey.status === 'enabled' ? 'text-orange-600' : 'text-green-600'}
          >
            {apiKey.status === 'enabled' ? (
              <>
                <IconUserOff className='mr-2 h-4 w-4' />
                {t('apikeys.actions.disable')}
              </>
            ) : (
              <>
                <IconUserCheck className='mr-2 h-4 w-4' />
                {t('apikeys.actions.enable')}
              </>
            )}
          </DropdownMenuItem>
        </DropdownMenuContent>
    </DropdownMenu>
  )
}