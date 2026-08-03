import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import Dashboard from '@/features/dashboard';

function ProtectedDashboard() {
  return (
    <RouteGuard requiredScopes={['read_dashboard']} scopeLevel="system">
      <Dashboard />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/')({
  component: ProtectedDashboard,
});
