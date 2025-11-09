import { useTranslation } from 'react-i18next'
import { useTopProjects } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'
import { FolderIcon } from 'lucide-react'
import { formatNumber } from '@/utils/format-number'

export function TopProjects() {
  const { t } = useTranslation()
  const { data: topProjects, isLoading, error } = useTopProjects(5)

  if (isLoading) {
    return (
      <div className="space-y-8">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center">
            <Skeleton className="h-9 w-9 rounded-md" />
            <div className="ml-4 space-y-1">
              <Skeleton className="h-4 w-[120px]" />
              <Skeleton className="h-3 w-[160px]" />
            </div>
            <Skeleton className="ml-auto h-4 w-[60px]" />
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-red-500 text-sm">
        {t('dashboard.charts.errorLoadingTopProjects')} {error.message}
      </div>
    )
  }

  if (!topProjects || topProjects.length === 0) {
    return (
      <div className="text-muted-foreground text-sm">
        {t('dashboard.charts.noProjectData')}
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {topProjects.map((project) => (
        <div key={project.projectId} className="flex items-center">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10">
            <FolderIcon className="h-5 w-5 text-primary" />
          </div>
          <div className="ml-4 space-y-1">
            <p className="text-sm font-medium leading-none">{project.projectName}</p>
            <p className="text-sm text-muted-foreground">
              {project.projectDescription}
            </p>
          </div>
          <div className="ml-auto font-medium">
            {formatNumber(project.requestCount)} {t('dashboard.stats.requests')}
          </div>
        </div>
      ))}
    </div>
  )
}
