import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import Playground from '@/features/playground';

function ProtectedPlayground() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['write_requests']}>
        <Playground />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/playground/')({
  component: ProtectedPlayground,
});
