import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import Users from '@/features/users';

function ProtectedUsers() {
  return (
    <RouteGuard requiredScopes={['read_users']}>
      <Users />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/users/')({
  component: ProtectedUsers,
});
