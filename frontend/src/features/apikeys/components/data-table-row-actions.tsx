import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { IconUserOff, IconUserCheck } from '@tabler/icons-react'
import { Row } from '@tanstack/react-table'
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
  const { openDialog } = useApiKeysContext()
  const apiKey = row.original

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
        <DropdownMenuItem onClick={() => openDialog('edit', apiKey)}>
          编辑
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => openDialog('status', apiKey)}
          className={apiKey.status === 'enabled' ? 'text-red-500!' : 'text-green-500!'}
        >
          {apiKey.status === 'enabled' ? '停用' : '激活'}
          <DropdownMenuShortcut>
            {apiKey.status === 'enabled' ? <IconUserOff size={16} /> : <IconUserCheck size={16} />}
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}