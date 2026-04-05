import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import { AgentFormPage } from '@/features/agents/components/agent-form-page';

function ProtectedAgentEdit() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['write_agents']}>
        <AgentFormPage mode='edit' />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/agents/$agentId/edit')({
  component: ProtectedAgentEdit,
});
