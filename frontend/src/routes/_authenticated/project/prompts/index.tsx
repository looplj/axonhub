import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import PromptsManagement from '@/features/prompts';

function ProtectedProjectPrompts() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_prompts']}>
        <PromptsManagement />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/prompts/')({
  component: ProtectedProjectPrompts,
});
