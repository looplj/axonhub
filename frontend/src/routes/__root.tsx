import { useEffect } from 'react';
import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools';
import { Toaster } from '@/components/ui/sonner';
import { CommandMenu } from '@/components/command-menu';
import { InitializationGuard } from '@/components/initialization-guard';
import { NavigationProgress } from '@/components/navigation-progress';
import { useBrandSettings } from '@/features/system/data/system';
import GeneralError from '@/features/errors/general-error';
import NotFoundError from '@/features/errors/not-found-error';
import { useAuthStore } from '@/stores/authStore';

function DocumentTitleSync() {
  const accessToken = useAuthStore((state) => state.auth.accessToken);
  const hasToken = Boolean(accessToken);
  const { data: brandSettings } = useBrandSettings({ enabled: hasToken });

  useEffect(() => {
    document.title = (hasToken && brandSettings?.title) || 'AxonHub';
  }, [brandSettings?.title, hasToken]);

  return null;
}

export const Route = createRootRouteWithContext<{
  queryClient: QueryClient;
}>()({
  component: () => {
    return (
      <>
        <DocumentTitleSync />
        <NavigationProgress />
        <InitializationGuard>
          <Outlet />
        </InitializationGuard>
        <CommandMenu />
        <Toaster duration={3000} />
        {/* {import.meta.env.MODE === 'development' && (
          <>
            <ReactQueryDevtools buttonPosition='bottom-left' />
            <TanStackRouterDevtools position='bottom-right' />
          </>
        )} */}
      </>
    );
  },
  notFoundComponent: NotFoundError,
  errorComponent: GeneralError,
});
