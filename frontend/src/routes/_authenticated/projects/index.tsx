import { createFileRoute } from '@tanstack/react-router'
import { RouteGuard } from '@/components/route-guard'
import Projects from '@/features/projects'

function ProtectedProjects() {
  return (
    <RouteGuard requiredScopes={['read_projects']}>
      <Projects />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/projects/')({
  component: ProtectedProjects,
})
