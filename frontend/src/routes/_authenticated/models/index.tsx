import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import ModelsManagement from '@/features/models';

function ProtectedModels() {
  return (
    <RouteGuard requiredScopes={['read_channels']} scopeLevel="system">
      <ModelsManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/models/')({
  component: ProtectedModels,
});
