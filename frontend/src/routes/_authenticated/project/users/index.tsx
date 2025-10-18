import { createFileRoute } from '@tanstack/react-router'
import { RouteGuard } from '@/components/route-guard'
import ProjectUsers from '@/features/proejct-users'

function ProtectedProjectUsers() {
  return (
    <RouteGuard requiredScopes={['read_users']}>
      <ProjectUsers />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/users/')({
  component: ProtectedProjectUsers,
})
