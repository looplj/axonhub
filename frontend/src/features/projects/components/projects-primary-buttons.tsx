import { IconPlus } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { PermissionGuard } from '@/components/permission-guard'
import { useProjectsContext } from '../context/projects-context'

export function ProjectsPrimaryButtons() {
  const { t } = useTranslation()
  const { setIsCreateDialogOpen } = useProjectsContext()

  return (
    <div className='flex items-center space-x-2'>
      {/* Create Project - requires write_projects permission */}
      <PermissionGuard requiredScope='write_projects'>
        <Button onClick={() => setIsCreateDialogOpen(true)}>
          <IconPlus className='mr-2 h-4 w-4' />
          {t('projects.createProject')}
        </Button>
      </PermissionGuard>
    </div>
  )
}
