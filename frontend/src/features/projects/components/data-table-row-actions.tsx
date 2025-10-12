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
import { Project } from '../data/schema'
import { useProjectsContext } from '../context/projects-context'

interface DataTableRowActionsProps {
  row: Row<Project>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const project = row.original
  const { setEditingProject, setArchivingProject, setActivatingProject } = useProjectsContext()
  const { projectPermissions } = usePermissions()
  
  // Don't show menu if user has no permissions
  if (!projectPermissions.canWrite) {
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
          <span className='sr-only'>{t('projects.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        {/* Edit - requires write permission */}
        {projectPermissions.canEdit && (
          <DropdownMenuItem onClick={() => setEditingProject(project)}>
            <IconEdit className='mr-2 h-4 w-4' />
            {t('projects.actions.edit')}
          </DropdownMenuItem>
        )}
        
        {projectPermissions.canEdit && projectPermissions.canWrite && (
          <DropdownMenuSeparator />
        )}
        
        {/* Archive - requires write permission, only for active projects */}
        {projectPermissions.canWrite && project.status === 'active' && (
          <DropdownMenuItem
            onClick={() => setArchivingProject(project)}
            className='text-destructive focus:text-destructive'
          >
            <IconTrash className='mr-2 h-4 w-4' />
            {t('projects.actions.archive')}
          </DropdownMenuItem>
        )}
        
        {/* Activate - requires write permission, only for archived projects */}
        {projectPermissions.canWrite && project.status === 'archived' && (
          <DropdownMenuItem
            onClick={() => setActivatingProject(project)}
          >
            <IconEdit className='mr-2 h-4 w-4' />
            {t('projects.actions.activate')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
