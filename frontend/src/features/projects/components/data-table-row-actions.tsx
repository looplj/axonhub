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
import { useProjectsContext } from '../context/projects-context';
import { Project } from '../data/schema';

interface DataTableRowActionsProps {
  row: Row<Project>;
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const project = row.original;
  const { setEditingProject, setArchivingProject, setActivatingProject } = useProjectsContext();
  const { projectPermissions } = usePermissions();
  const [open, setOpen] = React.useState(false);

  // Don't show menu if user has no permissions
  if (!projectPermissions.canWrite) {
    return null;
  }

  const handleEdit = () => {
    setOpen(false);
    setTimeout(() => setEditingProject(project), 0);
  };

  const handleArchive = () => {
    setOpen(false);
    setTimeout(() => setArchivingProject(project), 0);
  };

  const handleActivate = () => {
    setOpen(false);
    setTimeout(() => setActivatingProject(project), 0);
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0'>
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('common.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        {/* Edit - requires write permission */}
        {projectPermissions.canEdit && (
          <DropdownMenuItem onClick={handleEdit}>
            <IconEdit className='mr-2 h-4 w-4' />
            {t('common.actions.edit')}
          </DropdownMenuItem>
        )}

        {projectPermissions.canEdit && projectPermissions.canWrite && <DropdownMenuSeparator />}

        {/* Archive - requires write permission, only for active projects */}
        {projectPermissions.canWrite && project.status === 'active' && (
          <DropdownMenuItem onClick={handleArchive} className='text-destructive focus:text-destructive'>
            <IconTrash className='mr-2 h-4 w-4' />
            {t('common.buttons.archive')}
          </DropdownMenuItem>
        )}

        {/* Activate - requires write permission, only for archived projects */}
        {projectPermissions.canWrite && project.status === 'archived' && (
          <DropdownMenuItem onClick={handleActivate}>
            <IconEdit className='mr-2 h-4 w-4' />
            {t('common.buttons.activate')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
