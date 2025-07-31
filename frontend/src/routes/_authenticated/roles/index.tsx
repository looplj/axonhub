import { createFileRoute } from '@tanstack/react-router'
import Roles from '@/features/roles'
import { RouteGuard } from '@/components/route-guard'

function ProtectedRoles() {
  return (
    <RouteGuard requiredScopes={['read_roles']}>
      <Roles />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/roles/')({
  component: ProtectedRoles,
})
