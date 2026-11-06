import { useEffect } from 'react';
import Cookies from 'js-cookie';
import { Outlet, useLocation } from '@tanstack/react-router';
import { resolveNavModule } from '@/config/nav-items';
import { useSidebarPrefsStore } from '@/stores/sidebarPrefsStore';
import { cn } from '@/lib/utils';
import { useVersionCheck } from '@/hooks/use-version-check';
import { SidebarProvider } from '@/components/ui/sidebar';
import { AppHeader } from '@/components/layout/app-header';
import { AppSidebar } from '@/components/layout/app-sidebar';
import { useEnsureSelectedProject } from '@/components/layout/project-switcher';
import SkipToMain from '@/components/skip-to-main';
import { OnboardingProvider } from '@/features/onboarding';
import { useSidebarData } from './sidebar';

interface Props {
  children?: React.ReactNode;
}

export function AuthenticatedLayout({ children }: Props) {
  const defaultOpen = Cookies.get('sidebar_state') !== 'false';
  const sidebarData = useSidebarData();
  const location = useLocation();
  const recordModuleVisit = useSidebarPrefsStore((state) => state.recordModuleVisit);

  useEnsureSelectedProject();

  // Remember the last page of each module so its header tab can return there.
  useEffect(() => {
    const moduleId = resolveNavModule(location.pathname);
    if (moduleId) {
      recordModuleVisit(moduleId, location.href);
    }
  }, [location.pathname, location.href, recordModuleVisit]);

  // Check for new version on mount (only for owners)
  useVersionCheck();

  return (
    <SidebarProvider defaultOpen={defaultOpen} className='fixed inset-0 min-h-0 flex-col overflow-hidden'>
      <AppHeader sidebarData={sidebarData} />
      <div className='flex flex-1 overflow-hidden'>
        <SkipToMain />
        <AppSidebar sidebarData={sidebarData} />
        <div
          id='content'
          className={cn(
            'ml-auto w-full max-w-full',
            'peer-data-[state=collapsed]:w-[calc(100%-var(--sidebar-width-icon)-1rem)]',
            'peer-data-[state=expanded]:w-[calc(100%-var(--sidebar-width))]',
            'sm:transition-[width] sm:duration-200 sm:ease-linear',
            'flex min-h-0 min-w-0 flex-1 flex-col overflow-auto pt-14 has-[main.fixed-main]:overflow-hidden',
            'group-data-[scroll-locked=1]/body:h-full',
            'has-[main.fixed-main]:group-data-[scroll-locked=1]/body:h-svh'
          )}
        >
          <OnboardingProvider>{children ? children : <Outlet />}</OnboardingProvider>
        </div>
      </div>
    </SidebarProvider>
  );
}
