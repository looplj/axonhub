import * as React from 'react';
import { ChevronsUpDown, FolderKanban } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useProjectStore } from '@/stores/projectStore';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useMyProjects } from '@/features/projects/data/projects';

export function ProjectSwitcher() {
  const { data: myProjects, isLoading: isLoadingProjects } = useMyProjects();
  const { t } = useTranslation();
  const { selectedProjectId, setSelectedProjectId } = useProjectStore();

  // 当项目列表加载完成后，验证并设置选中的项目
  React.useEffect(() => {
    // 如果项目列表还在加载，不做任何操作
    if (!myProjects) {
      return;
    }

    // 如果用户没有任何项目，清空选中的项目
    if (myProjects.length === 0) {
      if (selectedProjectId) {
        setSelectedProjectId(null);
      }
      return;
    }

    // 如果已有选中的项目且在列表中存在，保持选中状态
    if (selectedProjectId) {
      const projectExists = myProjects.some((p) => p.id === selectedProjectId);
      if (projectExists) {
        return;
      }
    }

    // 只有在以下情况才选择第一个项目：
    // 1. 没有选中的项目（首次访问）
    // 2. 选中的项目不在当前列表中（项目被删除或用户被移除）
    const firstProject = myProjects[0];
    setSelectedProjectId(firstProject.id);
  }, [myProjects, selectedProjectId, setSelectedProjectId]);

  // 处理项目切换
  const handleProjectChange = (projectId: string) => {
    setSelectedProjectId(projectId);
  };

  // 获取当前选中的项目
  const selectedProject = React.useMemo(() => {
    return myProjects?.find((p) => p.id === selectedProjectId);
  }, [myProjects, selectedProjectId]);

  // 是否有项目可以切换
  const hasProjects = !isLoadingProjects && myProjects && myProjects.length > 0;

  if (!hasProjects) {
    return null;
  }

  const displayName = selectedProject?.name || t('sidebar.projectSwitcher.selectProject');

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button className='hover:bg-accent/50 inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm leading-none transition-colors'>
          <span className='text-sm leading-none font-medium'>{displayName}</span>
          <ChevronsUpDown className='text-muted-foreground size-3' />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className='min-w-56 rounded-lg' align='start' sideOffset={4}>
        <DropdownMenuLabel className='text-muted-foreground text-xs'>{t('sidebar.projectSwitcher.projects')}</DropdownMenuLabel>
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
  );
}
