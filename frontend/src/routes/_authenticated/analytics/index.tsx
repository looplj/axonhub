import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import AnalyticsPage from '@/features/analytics';

function ProtectedAnalytics() {
  return (
    <RouteGuard requiredScopes={['read_dashboard']} scopeLevel="system">
      <AnalyticsPage />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/analytics/')({
  component: ProtectedAnalytics,
});
