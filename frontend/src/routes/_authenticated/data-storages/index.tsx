import { createFileRoute } from '@tanstack/react-router'
import DataStoragesManagement from '@/features/data-storages'
import { RouteGuard } from '@/components/route-guard'

function ProtectedDataStorages() {
  return (
    <RouteGuard requiredScopes={['write_data_storages']}>
      <DataStoragesManagement />
    </RouteGuard>
  )
}

export const Route = createFileRoute('/_authenticated/data-storages/')({
  component: ProtectedDataStorages,
})
