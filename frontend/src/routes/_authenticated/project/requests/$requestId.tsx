import { createFileRoute } from '@tanstack/react-router'
import RequestDetailPage from '@/features/requests/components/request-detail-page'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedRequestDetail() {
  return (
    <ProjectGuard>
      <RequestDetailPage />
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/requests/$requestId')({  
  component: ProtectedRequestDetail,
})