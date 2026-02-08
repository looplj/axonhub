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
              <Link to='/system'>
                <IconSettings className='h-4 w-4' />
                <span>{t('sidebar.settings')}</span>
              </Link>
            </SidebarMenuButton>
          </PermissionGuard>
        </SidebarMenuItem>
      </SidebarMenu>

      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton asChild>
            <div className='flex items-center gap-2'>
              <LanguageSwitch />
              <span>{t('language.toggle')}</span>
            </div>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>

      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton asChild>
            <div className='flex items-center gap-2'>
              <ThemeSwitch />
              <span>{t('theme.toggle')}</span>
            </div>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </>
  );
}
