import { createFileRoute } from '@tanstack/react-router'
import ApiKeysManagement from '@/features/apikeys'

function ProtectedApiKeys() {
  return <ApiKeysManagement />
}

export const Route = createFileRoute('/_authenticated/api-keys/')({
  component: ProtectedApiKeys,
})
