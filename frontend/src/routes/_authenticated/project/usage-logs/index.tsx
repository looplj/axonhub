import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import UsageLogsManagement from '@/features/usage-logs';

function ProtectedProjectUsageLogs() {
  return (
    <ProjectGuard>
      <UsageLogsManagement />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/usage-logs/')({
  component: ProtectedProjectUsageLogs,
});
