import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import ChannelsManagement from '@/features/channels';

function ProtectedChannels() {
  return (
    <RouteGuard requiredScopes={['read_channels']} scopeLevel="system">
      <ChannelsManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/channels/')({
  component: ProtectedChannels,
});
