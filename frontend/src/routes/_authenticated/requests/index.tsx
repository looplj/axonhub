import { createFileRoute } from '@tanstack/react-router'
import RequestsManagement from '@/features/requests'

export const Route = createFileRoute('/_authenticated/requests/')({
  component: RequestsManagement,
})