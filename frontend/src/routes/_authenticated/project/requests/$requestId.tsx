import { createFileRoute } from '@tanstack/react-router'
import RequestDetailPage from '@/features/requests/components/request-detail-page'

export const Route = createFileRoute('/_authenticated/project/requests/$requestId')({  
  component: RequestDetailPage,
})