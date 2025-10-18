import { createFileRoute } from '@tanstack/react-router'
import { RouteGuard } from '@/components/route-guard'
import Roles from '@/features/project-roles'

function ProtectedProjectRoles() {
  return (
    <RouteGuard requiredScopes={['read_roles']}>
      <Roles />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/roles/')({
  component: ProtectedProjectRoles,
})
