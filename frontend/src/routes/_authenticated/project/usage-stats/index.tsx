import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import UsageStatisticsPage from '@/features/usage-statistics';

function ProtectedUsageStats() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel="any" requireProjectOwner={true}>
        <UsageStatisticsPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/usage-stats/')({
  component: ProtectedUsageStats,
});
