import { createFileRoute } from '@tanstack/react-router'
import SystemManagement from '@/features/system'
import { RouteGuard } from '@/components/route-guard'

function ProtectedSystem() {
  return (
    <RouteGuard requiredScopes={['read_system']}>
      <SystemManagement />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/system/')({
  component: ProtectedSystem,
})