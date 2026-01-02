import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import Roles from '@/features/project-roles';

function ProtectedProjectRoles() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_roles']}>
        <Roles />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/roles/')({
  component: ProtectedProjectRoles,
});
