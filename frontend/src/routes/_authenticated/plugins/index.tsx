import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import PluginsManagement from '@/features/plugins';

function ProtectedPlugins() {
  return (
    <RouteGuard requiredScopes={['read_plugins']} scopeLevel='system'>
      <PluginsManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/plugins/')({
  component: ProtectedPlugins,
});
