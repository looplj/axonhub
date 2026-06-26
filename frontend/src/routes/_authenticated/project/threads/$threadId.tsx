import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import { ThreadDetailPage } from '@/features/threads/components';

function ProtectedThreadDetail() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel="any">
        <ThreadDetailPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/threads/$threadId')({
  component: ProtectedThreadDetail,
});
