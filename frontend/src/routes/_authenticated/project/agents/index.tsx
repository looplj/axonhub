import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import AgentsManagement from '@/features/agents';

function ProtectedProjectAgents() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_agents']}>
        <AgentsManagement />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/agents/')({
  component: ProtectedProjectAgents,
});

