import { DotsHorizontalIcon } from '@radix-ui/react-icons'
import { Row } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { IconEdit, IconTrash } from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { usePermissions } from '@/hooks/usePermissions'
import { Role } from '../data/schema'
import { useRolesContext } from '../context/roles-context'

interface DataTableRowActionsProps {
  row: Row<Role>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const role = row.original
  const { setEditingRole, setDeletingRole } = useRolesContext()
  const { rolePermissions } = usePermissions()
  
  // Don't show menu if user has no permissions
  if (!rolePermissions.canWrite) {
    return null
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant='ghost'
          className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'
        >
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('roles.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        {/* Edit - requires write permission */}
        {rolePermissions.canEdit && (
          <DropdownMenuItem onClick={() => setEditingRole(role)}>
            <IconEdit className='mr-2 h-4 w-4' />
            {t('roles.actions.edit')}
          </DropdownMenuItem>
        )}
        
        {/* Separator only if there are both edit and delete actions */}
        {rolePermissions.canEdit && rolePermissions.canDelete && (
          <DropdownMenuSeparator />
        )}
        
        {/* Delete - requires write permission */}
        {rolePermissions.canDelete && (
          <DropdownMenuItem
            onClick={() => setDeletingRole(role)}
            className='text-destructive focus:text-destructive'
          >
            <IconTrash className='mr-2 h-4 w-4' />
            {t('roles.actions.delete')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}