import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import RequestDetailPage from '@/features/requests/components/request-detail-page';

function ProtectedRequestDetail() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel="any">
        <RequestDetailPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/requests/$requestId')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: ProtectedRequestDetail,
});
