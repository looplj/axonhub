import { Link } from '@tanstack/react-router';
import { IconSettings } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from '@/components/ui/sidebar';
import { LanguageSwitch } from '@/components/language-switch';
import { PermissionGuard } from '@/components/permission-guard';
import { ThemeSwitch } from '@/components/theme-switch';

export function MobileHeaderControls() {
  const { t } = useTranslation();

  return (
    <>
      <SidebarMenu>
        <SidebarMenuItem>
          <PermissionGuard requiredSystemScope='read_system'>
            <SidebarMenuButton asChild>
              <Link to='/system' data-testid='mobile-profile-button'>
                <IconSettings className='h-4 w-4' />
                <span>{t('sidebar.settings')}</span>
              </Link>
            </SidebarMenuButton>
          </PermissionGuard>
        </SidebarMenuItem>
      </SidebarMenu>

      <SidebarMenu>
        <SidebarMenuItem>
          <div className='flex items-center justify-between px-2 py-1.5' data-testid='mobile-language-switch'>
            <span className='text-sm'>{t('language.toggle')}</span>
            <LanguageSwitch />
          </div>
        </SidebarMenuItem>
      </SidebarMenu>

      <SidebarMenu>
        <SidebarMenuItem>
          <div className='flex items-center justify-between px-2 py-1.5' data-testid='mobile-theme-switch'>
            <span className='text-sm'>{t('theme.toggle')}</span>
            <ThemeSwitch />
          </div>
        </SidebarMenuItem>
      </SidebarMenu>
    </>
  );
}
