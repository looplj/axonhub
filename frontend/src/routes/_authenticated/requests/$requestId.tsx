import { createFileRoute } from '@tanstack/react-router'
import RequestDetailPage from '@/features/requests/components/request-detail-page'

export const Route = createFileRoute('/_authenticated/requests/$requestId')({  
  component: RequestDetailPage,
})