import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import SystemManagement from '@/features/system';

type SystemTabKey = 'brand' | 'storage' | 'retry' | 'webhook' | 'about' | 'general' | 'proxy' | 'backup';

function ProtectedSystem() {
  const search = Route.useSearch();

  return (
    <RouteGuard requiredScopes={['read_system']}>
      <SystemManagement initialTab={search.tab as SystemTabKey | undefined} />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/system/')({
  component: ProtectedSystem,
  validateSearch: (search: { tab?: SystemTabKey }) => search,
});
