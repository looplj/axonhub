import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import { AgentChatPage } from '@/features/agents/components/agent-chat-page';

function ProtectedAgentThreadChat() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_agents']}>
        <AgentChatPage />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/agents/$agentId/threads/$threadId')({
  component: ProtectedAgentThreadChat,
});

