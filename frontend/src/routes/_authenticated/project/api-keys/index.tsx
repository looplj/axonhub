import { createFileRoute } from '@tanstack/react-router'
import ApiKeys from '@/features/apikeys'
import { RouteGuard } from '@/components/route-guard'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedProjectApiKeys() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_api_keys']}>
        <ApiKeys />
      </RouteGuard>
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/api-keys/')({
  component: ProtectedProjectApiKeys,
})
