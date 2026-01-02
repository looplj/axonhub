import { Outlet } from '@tanstack/react-router';
import { IconBrowserCheck, IconNotification, IconPalette, IconTool, IconUser } from '@tabler/icons-react';
import { Main } from '@/components/layout/main';

export default function Settings() {
  return (
    <>
      <Main fixed>
        {/* <div className='space-y-0.5'>
          <h1 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('settings.title')}
          </h1>
          <p className='text-muted-foreground'>
            {t('settings.description')}
          </p>
        </div> */}
        {/* <Separator className='my-4 lg:my-6' /> */}
        <div className='flex flex-1 flex-col space-y-2 overflow-hidden md:space-y-2 lg:flex-row lg:space-y-0 lg:space-x-12'>
          {/* <aside className='top-0 lg:sticky lg:w-1/5'>
            <SidebarNav items={sidebarNavItems} />
          </aside> */}
          <div className='flex w-full overflow-y-hidden p-1'>
            <Outlet />
          </div>
        </div>
      </Main>
    </>
  );
}

const sidebarNavItems = [
  {
    title: 'settings.nav.profile',
    icon: <IconUser size={18} />,
    href: '/settings/profile',
  },
  {
    title: 'settings.nav.appearance',
    icon: <IconPalette size={18} />,
    href: '/settings/appearance',
  },
  {
    title: 'settings.nav.notifications',
    icon: <IconNotification size={18} />,
    href: '/settings/notifications',
  },
  {
    title: 'settings.nav.display',
    icon: <IconBrowserCheck size={18} />,
    href: '/settings/display',
  },
];
