import { createFileRoute } from '@tanstack/react-router'
import TraceDetailPage from '@/features/traces/components/trace-detail-page'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedTraceDetail() {
  return (
    <ProjectGuard>
      <TraceDetailPage />
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/traces/$traceId')({
  component: ProtectedTraceDetail,
})
