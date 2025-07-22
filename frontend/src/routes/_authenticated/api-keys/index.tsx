import { createFileRoute } from '@tanstack/react-router'
import ApiKeysManagement from '@/features/apikeys'
  
export const Route = createFileRoute('/_authenticated/api-keys/')({
  component: ApiKeysManagement,
})