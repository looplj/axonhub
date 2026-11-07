import { useState, useCallback } from 'react';
import { Link } from '@tanstack/react-router';
import { IconSettings } from '@tabler/icons-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { SidebarTrigger, useSidebar } from '@/components/ui/sidebar';
import { Button } from '@/components/ui/button';
import { LanguageSwitch } from '@/components/language-switch';
import { ThemeSwitch } from '@/components/theme-switch';
import { PluginHeaderActions } from '@/features/plugins/components/plugin-header-actions';
import { QuotaBadges } from '@/components/quota-badges';
import { PermissionGuard } from '@/components/permission-guard';
import { checkProviderQuotas } from '@/features/system/data/quotas';
import { useBrandSettings } from '@/features/system/data/system';
import { BrandLogo } from './brand-logo';
import { NavModuleTabs } from './nav-module-tabs';
import { ProjectSwitcher } from './project-switcher';
import { type SidebarData } from './types';
import { toast } from 'sonner';

export function AppHeader({ sidebarData }: { sidebarData: SidebarData }) {
  const { data: brandSettings } = useBrandSettings();
  const { t } = useTranslation();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const queryClient = useQueryClient();
  const { isMobile } = useSidebar();
  const displayName = brandSettings?.brandName || 'AxonHub';
  const showProjectSwitcher = sidebarData.activeModuleId === 'project';

  const refreshMutation = useMutation({
    mutationFn: async () => {
      return checkProviderQuotas();
    },
    onSuccess: () => {
      void queryClient.refetchQueries({ queryKey: ['provider-quotas'] });
      toast.success(t('system.providerQuota.refresh.success'));
    },
    onError: (error: any) => {
      toast.error(error.message || t('system.providerQuota.refresh.failure'));
    },
  });

  const handleRefresh = useCallback(() => {
    setIsRefreshing(true);
    refreshMutation.mutate(undefined, {
      onSettled: () => setIsRefreshing(false),
    });
  }, [refreshMutation]);

  return (
    <header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 fixed top-0 z-50 w-full backdrop-blur'>
      <div className='flex h-14 items-center justify-between'>
        {/* Logo + Project Switcher - 左侧对齐 */}
        <div className='flex items-center gap-2 pl-6'>
          {/* Sidebar Toggle - 与侧边栏图标垂直对齐 */}
          <SidebarTrigger className='-ml-4 size-8' />

          {/* Logo */}
          <div className='flex items-center gap-2'>
            <div className='flex size-8 shrink-0 items-center justify-center overflow-hidden rounded'>
              <BrandLogo brandLogo={brandSettings?.brandLogo} />
            </div>
            <span className='text-sm leading-none font-semibold'>{displayName}</span>
          </div>

          {/* Module tabs - desktop only, the mobile sidebar has its own switcher */}
          {!isMobile && (
            <NavModuleTabs modules={sidebarData.modules} activeModuleId={sidebarData.activeModuleId} className='ml-4' />
          )}

          {/* Project Switcher - only meaningful for project-scoped pages. Placed after
              the tabs so toggling it never shifts them. */}
          {showProjectSwitcher && (
            <>
              <span className='text-muted-foreground/60 mx-0.5 text-sm select-none'>/</span>
              <ProjectSwitcher />
            </>
          )}
        </div>

        {/* 右侧控件 */}
        <div className='flex items-center gap-2 pr-6'>
          {/* Quota Badges - only visible to users with channel read permission */}
          <PermissionGuard requiredSystemScope='read_channels'>
            <QuotaBadges onRefresh={handleRefresh} isRefreshing={isRefreshing} />
          </PermissionGuard>

          <PluginHeaderActions isMobile={isMobile} />

          {/* Desktop-only controls - hidden on mobile */}
          {!isMobile && (
            <>
              <PermissionGuard requiredSystemScope='read_settings'>
                <Link to='/system'>
                  <Button variant='ghost' size='icon' className='size-8'>
                    <IconSettings className='h-4 w-4' />
                  </Button>
                </Link>
              </PermissionGuard>
              <LanguageSwitch />
              <ThemeSwitch />
            </>
          )}
        </div>
      </div>
    </header>
  );
}
