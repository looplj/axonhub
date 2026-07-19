import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import TraceDetailPage from '@/features/traces/components/trace-detail-page';

function ProtectedTraceDetail() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel="any">
        <TraceDetailPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/traces/$traceId')({
  component: ProtectedTraceDetail,
});
