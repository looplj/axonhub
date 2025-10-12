import * as React from 'react'
import { ChevronsUpDown, FolderKanban } from 'lucide-react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useMyProjects } from '@/features/projects/data/projects'
import { useTranslation } from 'react-i18next'

const PROJECT_STORAGE_KEY = 'axonhub_selected_project_id'

export function ProjectSwitcher() {
  const { data: myProjects, isLoading: isLoadingProjects } = useMyProjects()
  const { t } = useTranslation()

  // 管理选中的项目
  const [selectedProjectId, setSelectedProjectId] = React.useState<string | null>(() => {
    return localStorage.getItem(PROJECT_STORAGE_KEY)
  })

  // 当项目列表加载完成后，验证并设置选中的项目
  React.useEffect(() => {
    if (!myProjects || myProjects.length === 0) return

    const savedProjectId = localStorage.getItem(PROJECT_STORAGE_KEY)
    const projectExists = myProjects.some(p => p.id === savedProjectId)

    if (!savedProjectId || !projectExists) {
      const firstProject = myProjects[0]
      setSelectedProjectId(firstProject.id)
      localStorage.setItem(PROJECT_STORAGE_KEY, firstProject.id)
    } else {
      setSelectedProjectId(savedProjectId)
    }
  }, [myProjects])

  // 处理项目切换
  const handleProjectChange = (projectId: string) => {
    setSelectedProjectId(projectId)
    localStorage.setItem(PROJECT_STORAGE_KEY, projectId)
  }

  // 获取当前选中的项目
  const selectedProject = React.useMemo(() => {
    return myProjects?.find(p => p.id === selectedProjectId)
  }, [myProjects, selectedProjectId])

  // 是否有项目可以切换
  const hasProjects = !isLoadingProjects && myProjects && myProjects.length > 0

  if (!hasProjects) {
    return null
  }

  const displayName = selectedProject?.name || t('sidebar.projectSwitcher.selectProject')

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button className='inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm hover:bg-accent/50 transition-colors leading-none'>
          <span className='font-medium text-sm leading-none'>{displayName}</span>
          <ChevronsUpDown className='size-3 text-muted-foreground' />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        className='min-w-56 rounded-lg'
        align='start'
        sideOffset={4}
      >
        <DropdownMenuLabel className='text-muted-foreground text-xs'>
          {t('sidebar.projectSwitcher.projects')}
        </DropdownMenuLabel>
        {myProjects.map((project) => (
          <DropdownMenuItem
            key={project.id}
            onClick={() => handleProjectChange(project.id)}
            className='gap-2 p-2'
          >
            <div className='flex size-6 items-center justify-center rounded-sm border'>
              <FolderKanban className='size-4 shrink-0' />
            </div>
            <div className='flex flex-col'>
              <span className='text-sm font-medium'>{project.name}</span>
              <span className='text-xs text-muted-foreground'>{project.slug}</span>
            </div>
            {selectedProjectId === project.id && (
              <DropdownMenuShortcut>✓</DropdownMenuShortcut>
            )}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
