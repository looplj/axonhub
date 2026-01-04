import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import TracesManagement from '@/features/traces';

function ProtectedProjectTraces() {
  return (
    <ProjectGuard>
      <TracesManagement />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/traces/')({
  component: ProtectedProjectTraces,
});
