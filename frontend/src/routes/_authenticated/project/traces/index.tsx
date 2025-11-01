import { createFileRoute } from '@tanstack/react-router'
import TracesManagement from '@/features/traces'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedProjectTraces() {
  return (
    <ProjectGuard>
      <TracesManagement />
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/traces/')({
  component: ProtectedProjectTraces,
})
