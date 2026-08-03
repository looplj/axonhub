import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import ApiKeysManagement from '@/features/apikeys';

function ProtectedApiKeys() {
  return (
    <RouteGuard requiredScopes={['read_api_keys']} scopeLevel="system">
      <ApiKeysManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/api-keys/')({
  component: ProtectedApiKeys,
});
