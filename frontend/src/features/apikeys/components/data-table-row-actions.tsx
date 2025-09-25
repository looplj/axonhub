import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { IconUserOff, IconUserCheck, IconEdit, IconSettings, IconArchive } from '@tabler/icons-react'
import { Row } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
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
    if (apiKey.status === 'archived') {
      // Archived API keys cannot be enabled/disabled
      return
    }
    openDialog('status', apiKey)
  }

  const handleArchive = (apiKey: ApiKey) => {
    openDialog('archive', apiKey)
  }

  const handleProfiles = (apiKey: ApiKey) => {
    openDialog('profiles', apiKey)
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
            <IconEdit className='mr-2 h-4 w-4' />
            {t('apikeys.actions.edit')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => handleProfiles(apiKey)}>
            <IconSettings className='mr-2 h-4 w-4' />
            {t('apikeys.actions.profiles')}
          </DropdownMenuItem>
          {apiKey.status !== 'archived' && (
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
          )}
          {apiKey.status !== 'archived' && (
            <DropdownMenuItem
              onClick={() => handleArchive(apiKey)}
              className='text-orange-600'
            >
              <IconArchive className='mr-2 h-4 w-4' />
              {t('apikeys.actions.archive')}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
    </DropdownMenu>
  )
}