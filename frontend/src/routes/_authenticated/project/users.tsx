import { createFileRoute } from '@tanstack/react-router'
import ProjectUsers from '@/features/proejct-users'
import { RouteGuard } from '@/components/route-guard'

function ProtectedProjectUsers() {
  return (
    <RouteGuard requiredScopes={['read_users']}>
      <ProjectUsers />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/users')({
  component: ProtectedProjectUsers,
})
