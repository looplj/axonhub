import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import RequestDetailPage from '@/features/requests/components/request-detail-page';

function ProtectedRequestDetail() {
  return (
    <ProjectGuard>
      <RequestDetailPage />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/requests/$requestId')({
  component: ProtectedRequestDetail,
});
