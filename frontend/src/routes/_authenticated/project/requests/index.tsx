import { createFileRoute } from '@tanstack/react-router'
import RequestsManagement from '@/features/requests'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedProjectRequests() {
  return (
    <ProjectGuard>
      <RequestsManagement />
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/requests/')({
  component: ProtectedProjectRequests,
})