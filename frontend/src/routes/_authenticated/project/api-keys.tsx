import { createFileRoute } from '@tanstack/react-router'
import ApiKeys from '@/features/apikeys'
import { RouteGuard } from '@/components/route-guard'

function ProtectedProjectApiKeys() {
  return (
    <RouteGuard requiredScopes={['read_api_keys']}>
      <ApiKeys />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/api-keys')({
  component: ProtectedProjectApiKeys,
})
