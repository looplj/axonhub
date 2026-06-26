import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import ThreadsManagement from '@/features/threads';

function ProtectedProjectThreads() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel="any">
        <ThreadsManagement />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/threads/')({
  component: ProtectedProjectThreads,
});
