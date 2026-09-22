import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import DashboardChannelSuccessRates from '@/features/dashboard/channel-success-rates';

function ProtectedDashboardChannelSuccessRates() {
  return (
    <RouteGuard requiredScopes={['read_dashboard']} scopeLevel="system">
      <DashboardChannelSuccessRates />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/dashboard/channel-success-rates')({
  component: ProtectedDashboardChannelSuccessRates,
});
