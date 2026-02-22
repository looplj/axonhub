import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import { AgentFormPage } from '@/features/agents/components/agent-form-page';

function ProtectedAgentCreate() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['write_agents']}>
        <AgentFormPage mode='create' />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/agents/create')({
  component: ProtectedAgentCreate,
});
