import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import PluginsManagement, { type PluginsTab } from '@/features/plugins';

function ProtectedPlugins() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  return (
    <RouteGuard requiredScopes={['read_plugins']} scopeLevel='system'>
      <PluginsManagement
        tab={search.tab ?? 'installed'}
        onTabChange={(tab) => navigate({ search: tab === 'installed' ? {} : { tab }, replace: true })}
      />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/plugins/')({
  component: ProtectedPlugins,
  validateSearch: (search: Record<string, unknown>): { tab?: PluginsTab } =>
    search.tab === 'market' || search.tab === 'installed' ? { tab: search.tab } : {},
});
