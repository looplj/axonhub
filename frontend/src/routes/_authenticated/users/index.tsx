import { createFileRoute } from '@tanstack/react-router'
import Users from '@/features/users'
import { RouteGuard } from '@/components/route-guard'

function ProtectedUsers() {
  return (
    <RouteGuard requiredScopes={['read_users']}>
      <Users />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/users/')({
  component: ProtectedUsers,
})
