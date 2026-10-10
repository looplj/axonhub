import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import SystemManagement from '@/features/system';
import type { SystemTabKey } from '@/features/system/components/tabs';

function ProtectedSystem() {
  const search = Route.useSearch();

  return (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel="system">
      <SystemManagement initialTab={search.tab as SystemTabKey | undefined} />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/system/')({
  component: ProtectedSystem,
  validateSearch: (search: { tab?: SystemTabKey }) => search,
});
