import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { ThreadDetailPage } from '@/features/threads/components';

function ProtectedThreadDetail() {
  return (
    <ProjectGuard>
      <ThreadDetailPage />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/threads/$threadId')({
  component: ProtectedThreadDetail,
});
