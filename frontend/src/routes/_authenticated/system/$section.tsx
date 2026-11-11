import { createFileRoute, redirect } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import SystemManagement from '@/features/system';
import type { SystemTabKey } from '@/features/system/components/tabs';
import { isSystemSubPageKey } from '@/features/system/pages';

function ProtectedSystemPage() {
  const { section } = Route.useParams();
  const search = Route.useSearch();
  if (!isSystemSubPageKey(section)) {
    return null;
  }

  return (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system'>
      <SystemManagement page={section} initialTab={search.tab} />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/system/$section')({
  beforeLoad: ({ params }) => {
    if (!isSystemSubPageKey(params.section)) {
      throw redirect({ to: '/system' });
    }
  },
  validateSearch: (search: { tab?: SystemTabKey }) => search,
  component: ProtectedSystemPage,
});
