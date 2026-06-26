import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import RequestsManagement from '@/features/requests';

function ProtectedProjectRequests() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel="any">
        <RequestsManagement />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/requests/')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: ProtectedProjectRequests,
});
