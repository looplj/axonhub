import React from 'react';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { Row } from '@tanstack/react-table';
import { IconEdit, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useRolesContext } from '../context/roles-context';
import { Role } from '../data/schema';

interface DataTableRowActionsProps {
  row: Row<Role>;
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const role = row.original;
  const { openDialog } = useRolesContext();
  const { rolePermissions } = usePermissions();
  const [open, setOpen] = React.useState(false);

  // Don't show menu if user has no permissions
  if (!rolePermissions.canWrite) {
    return null;
  }

  const handleEdit = () => {
    setOpen(false);
    // Use setTimeout to ensure dropdown closes before opening dialog
    setTimeout(() => openDialog('edit', role), 0);
  };

  const handleDelete = () => {
    setOpen(false);
    // Use setTimeout to ensure dropdown closes before opening dialog
    setTimeout(() => openDialog('delete', role), 0);
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0' data-testid='row-actions'>
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('common.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        {/* Edit - requires write permission */}
        {rolePermissions.canEdit && (
          <DropdownMenuItem onClick={handleEdit}>
            <IconEdit className='mr-2 h-4 w-4' />
            {t('common.actions.edit')}
          </DropdownMenuItem>
        )}

        {/* Separator only if there are both edit and delete actions */}
        {rolePermissions.canEdit && rolePermissions.canDelete && <DropdownMenuSeparator />}

        {/* Delete - requires write permission */}
        {rolePermissions.canDelete && (
          <DropdownMenuItem onClick={handleDelete} className='text-destructive focus:text-destructive'>
            <IconTrash className='mr-2 h-4 w-4' />
            {t('common.actions.delete')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
