import { createFileRoute } from '@tanstack/react-router'
import UsageLogsManagement from '@/features/usage-logs'
import { ProjectGuard } from '@/components/project-guard'

function ProtectedProjectUsageLogs() {
  return (
    <ProjectGuard>
      <UsageLogsManagement />
    </ProjectGuard>
  )
}

export const Route = createFileRoute('/_authenticated/project/usage-logs/')({
  component: ProtectedProjectUsageLogs,
})