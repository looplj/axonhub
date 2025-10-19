import * as React from 'react'
import { ChevronsUpDown, FolderKanban } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useProjectStore } from '@/stores/projectStore'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useMyProjects } from '@/features/projects/data/projects'

export function ProjectSwitcher() {
  const { data: myProjects, isLoading: isLoadingProjects } = useMyProjects()
  const { t } = useTranslation()
  const { selectedProjectId, setSelectedProjectId } = useProjectStore()

  // 当项目列表加载完成后，验证并设置选中的项目
  React.useEffect(() => {
    // 如果用户没有任何项目，清空选中的项目
    if (!myProjects || myProjects.length === 0) {
      if (selectedProjectId) {
        setSelectedProjectId(null)
      }
      return
    }

    const projectExists = myProjects.some((p) => p.id === selectedProjectId)

    if (!selectedProjectId || !projectExists) {
      const firstProject = myProjects[0]
      setSelectedProjectId(firstProject.id)
    }
  }, [myProjects, selectedProjectId, setSelectedProjectId])

  // 处理项目切换
  const handleProjectChange = (projectId: string) => {
    setSelectedProjectId(projectId)
  }

  // 获取当前选中的项目
  const selectedProject = React.useMemo(() => {
    return myProjects?.find((p) => p.id === selectedProjectId)
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
        <button className='hover:bg-accent/50 inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm leading-none transition-colors'>
          <span className='text-sm leading-none font-medium'>{displayName}</span>
          <ChevronsUpDown className='text-muted-foreground size-3' />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className='min-w-56 rounded-lg' align='start' sideOffset={4}>
        <DropdownMenuLabel className='text-muted-foreground text-xs'>
          {t('sidebar.projectSwitcher.projects')}
        </DropdownMenuLabel>
        {myProjects.map((project) => (
          <DropdownMenuItem key={project.id} onClick={() => handleProjectChange(project.id)} className='gap-2 p-2'>
            <div className='flex size-6 items-center justify-center rounded-sm border'>
              <FolderKanban className='size-4 shrink-0' />
            </div>
            <div className='flex flex-col'>
              <span className='text-sm font-medium'>{project.name}</span>
            </div>
            {selectedProjectId === project.id && <DropdownMenuShortcut>✓</DropdownMenuShortcut>}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
