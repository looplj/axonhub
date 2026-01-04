import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import ApiKeys from '@/features/apikeys';

function ProtectedProjectApiKeys() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_api_keys']}>
        <ApiKeys />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/api-keys/')({
  component: ProtectedProjectApiKeys,
});
