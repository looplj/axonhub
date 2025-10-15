import { createFileRoute } from '@tanstack/react-router'
import UsageLogsManagement from '@/features/usage-logs'

export const Route = createFileRoute('/_authenticated/project/usage-logs/')({
  component: UsageLogsManagement,
})