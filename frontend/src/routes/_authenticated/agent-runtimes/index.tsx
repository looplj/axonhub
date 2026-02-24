import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import AgentRuntimesManagement from '@/features/agent-runtimes';

function ProtectedAgentRuntimes() {
  return (
    <RouteGuard requiredScopes={['read_agents']}>
      <AgentRuntimesManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/agent-runtimes/')({
  component: ProtectedAgentRuntimes,
});
