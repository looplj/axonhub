import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import Roles from '@/features/roles';

function ProtectedRoles() {
  return (
    <RouteGuard requiredScopes={['read_roles']}>
      <Roles />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/roles/')({
  component: ProtectedRoles,
});
