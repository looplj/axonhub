import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import RequestsManagement from '@/features/requests';

function ProtectedProjectRequests() {
  return (
    <ProjectGuard>
      <RequestsManagement />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/requests/')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: ProtectedProjectRequests,
});
