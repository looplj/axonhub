import { createFileRoute } from '@tanstack/react-router'
import { RouteGuard } from '@/components/route-guard'
import { ProjectGuard } from '@/components/project-guard'
import ProjectUsers from '@/features/proejct-users'

function ProtectedProjectUsers() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_users']}>
        <ProjectUsers />
      </RouteGuard>
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/users/')({
  component: ProtectedProjectUsers,
})
