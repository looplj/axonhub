import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import MessageChannelsManagement from '@/features/message-channels';

function ProtectedProjectMessageChannels() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_agents']}>
        <MessageChannelsManagement />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/message-channels/')({
  component: ProtectedProjectMessageChannels,
});
