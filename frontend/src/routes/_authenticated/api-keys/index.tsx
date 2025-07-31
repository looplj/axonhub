import { createFileRoute } from '@tanstack/react-router'
import ApiKeysManagement from '@/features/apikeys'
import { RouteGuard } from '@/components/route-guard'

function ProtectedApiKeys() {
  return (
    <RouteGuard requiredScopes={['read_api_keys']}>
      <ApiKeysManagement />
    </RouteGuard>
  )
}
  
export const Route = createFileRoute('/_authenticated/api-keys/')({
  component: ProtectedApiKeys,
})