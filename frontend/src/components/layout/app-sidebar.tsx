import { Sidebar, SidebarContent, SidebarFooter, SidebarHeader, SidebarRail, useSidebar } from '@/components/ui/sidebar';
import { NavGroup } from '@/components/layout/nav-group';
import { NavUser } from '@/components/layout/nav-user';
import { MobileHeaderControls } from './mobile-header-controls';
import { NavModuleTabs } from './nav-module-tabs';
import { SidebarData } from './types';

type IProps = React.ComponentProps<typeof Sidebar> & {
  sidebarData: SidebarData;
};

export function AppSidebar({ sidebarData, ...props }: IProps) {
  const { isMobile } = useSidebar();

  return (
    <Sidebar collapsible='icon' variant='floating' {...props}>
      {isMobile && sidebarData.modules.length > 1 && (
        <SidebarHeader>
          <NavModuleTabs modules={sidebarData.modules} activeModuleId={sidebarData.activeModuleId} variant='sidebar' />
        </SidebarHeader>
      )}
      <SidebarContent>
        {sidebarData.navGroups.map((props) => (
          <NavGroup key={props.id ?? props.title} {...props} />
        ))}
      </SidebarContent>
      <SidebarFooter>
        {isMobile && <MobileHeaderControls />}
        <NavUser user={sidebarData.user} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
