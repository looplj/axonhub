import { createFileRoute } from '@tanstack/react-router'
import { RouteGuard } from '@/components/route-guard'
import Playground from '@/features/playground'

function ProtectedPlayground() {
  return (
    <RouteGuard requiredScopes={['write_requests']}>
      <Playground />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/playground/')({
  component: ProtectedPlayground,
})
