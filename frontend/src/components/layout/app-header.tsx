import { useState, useCallback } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { SidebarTrigger, useSidebar } from '@/components/ui/sidebar';
import { LanguageSwitch } from '@/components/language-switch';
import { ThemeSwitch } from '@/components/theme-switch';
import { QuotaBadges } from '@/components/quota-badges';
import { checkProviderQuotas } from '@/features/system/data/quotas';
import { useBrandSettings } from '@/features/system/data/system';
import { ProjectSwitcher } from './project-switcher';
import { toast } from 'sonner';

export function AppHeader() {
  const { data: brandSettings } = useBrandSettings();
  const { t } = useTranslation();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const queryClient = useQueryClient();
  const { isMobile } = useSidebar();
  const displayName = brandSettings?.brandName || 'AxonHub';

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
              {brandSettings?.brandLogo ? (
                <img
                  src={brandSettings.brandLogo}
                  alt='Brand Logo'
                  width={24}
                  height={24}
                  className='size-8 object-cover'
                  onError={(e) => {
                    e.currentTarget.src = '/logo.jpg';
                  }}
                />
              ) : (
                <img src='/logo.jpg' alt='Default Logo' width={24} height={24} className='size-8 object-cover' />
              )}
            </div>
            <span className='text-sm leading-none font-semibold'>{displayName}</span>
          </div>

          {/* Separator */}
          <div className='bg-border mx-0.5 h-3.5 w-px' />

          {/* Project Switcher */}
          <ProjectSwitcher />
        </div>

        {/* 右侧控件 */}
        <div className='flex items-center gap-2 pr-6'>
          {/* Quota Badges - always visible */}
          <QuotaBadges onRefresh={handleRefresh} isRefreshing={isRefreshing} />

          {/* Desktop-only controls - hidden on mobile */}
          {!isMobile && (
            <>
              <LanguageSwitch />
              <ThemeSwitch />
            </>
          )}
        </div>
      </div>
    </header>
  );
}
