import { useEffect } from 'react';
import { useRouter } from '@tanstack/react-router';
import { IconFolderOff, IconFolderPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { useMyProjects } from '@/features/projects/data/projects';

interface ProjectGuardProps {
  children: React.ReactNode;
  fallbackPath?: string;
  showNoProjectPage?: boolean;
}

export function ProjectGuard({ children, fallbackPath = '/projects', showNoProjectPage = true }: ProjectGuardProps) {
  const router = useRouter();
  const selectedProjectId = useSelectedProjectId();
  const { data: myProjects, isLoading } = useMyProjects();

  // 检查是否有选中的项目
  const hasSelectedProject = !!selectedProjectId;

  // 检查用户是否有任何项目
  const hasAnyProjects = !isLoading && myProjects && myProjects.length > 0;

  useEffect(() => {
    // 如果没有选中项目且不显示提示页面，则重定向
    if (!hasSelectedProject && !showNoProjectPage && !isLoading) {
      router.navigate({ to: fallbackPath });
    }
  }, [hasSelectedProject, showNoProjectPage, fallbackPath, router, isLoading]);

  // 加载中时不显示任何内容
  if (isLoading) {
    return null;
  }

  // 如果没有选中项目
  if (!hasSelectedProject) {
    if (showNoProjectPage) {
      return <NoProjectPage hasAnyProjects={!!hasAnyProjects} onGoToProjects={() => router.navigate({ to: fallbackPath })} />;
    }
    return null; // 重定向中，不显示任何内容
  }

  return <>{children}</>;
}

function NoProjectPage({ hasAnyProjects, onGoToProjects }: { hasAnyProjects: boolean; onGoToProjects: () => void }) {
  const { t } = useTranslation();

  return (
    <div className='flex h-screen items-center justify-center'>
      <div className='max-w-md text-center'>
        <div className='mb-6'>
          <IconFolderOff className='mx-auto h-16 w-16 text-orange-500' />
        </div>

        <Alert className='mb-6'>
          <IconFolderOff className='h-4 w-4' />
          <AlertTitle>{t('common.projectGuard.noProjectSelected')}</AlertTitle>
          <AlertDescription>
            {hasAnyProjects ? t('common.projectGuard.pleaseSelectProject') : t('common.projectGuard.pleaseJoinOrCreateProject')}
          </AlertDescription>
        </Alert>

        {/* <Button onClick={onGoToProjects} className="gap-2">
          <IconFolderPlus className="h-4 w-4" />
          {hasAnyProjects 
            ? t('common.projectGuard.goToProjects')
            : t('common.projectGuard.createOrJoinProject')
          }
        </Button> */}
      </div>
    </div>
  );
}
