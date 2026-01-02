import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import ThreadsManagement from '@/features/threads';

function ProtectedProjectThreads() {
  return (
    <ProjectGuard>
      <ThreadsManagement />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/threads/')({
  component: ProtectedProjectThreads,
});
