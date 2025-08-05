import { createFileRoute } from '@tanstack/react-router'
import SystemManagement from '@/features/system'

export const Route = createFileRoute('/_authenticated/system')({
  component: SystemManagement,
})