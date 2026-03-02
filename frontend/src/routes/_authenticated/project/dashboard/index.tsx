import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import { ProjectDashboardPage } from '@/features/dashboard';

function ProtectedProjectDashboard() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']}>
        <ProjectDashboardPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/dashboard/')({
  component: ProtectedProjectDashboard,
});
