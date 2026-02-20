import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import { AgentDetailPage } from '@/features/agents/components/agent-detail-page';

function ProtectedAgentDetail() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_agents']}>
        <AgentDetailPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/agents/$agentId')({
  component: ProtectedAgentDetail,
});

