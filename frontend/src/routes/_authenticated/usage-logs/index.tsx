import { createFileRoute } from '@tanstack/react-router'
import UsageLogsManagement from '@/features/usage-logs'

export const Route = createFileRoute('/_authenticated/usage-logs/')({
  component: UsageLogsManagement,
})