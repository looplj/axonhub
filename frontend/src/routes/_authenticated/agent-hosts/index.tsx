import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import AgentHostsManagement from '@/features/agent-hosts';

function ProtectedAgentHosts() {
  return (
    <RouteGuard requiredScopes={['read_agents']}>
      <AgentHostsManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/agent-hosts/')({
  component: ProtectedAgentHosts,
});
