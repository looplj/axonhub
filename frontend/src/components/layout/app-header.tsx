import { useBrandSettings } from '@/features/system/data/system'
import { LanguageSwitch } from '@/components/language-switch'
import { ThemeSwitch } from '@/components/theme-switch'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ProjectSwitcher } from './project-switcher'
import { SidebarTrigger } from '@/components/ui/sidebar'

export function AppHeader() {
  const { data: brandSettings } = useBrandSettings()
  const displayName = brandSettings?.brandName || 'AxonHub'

  return (
    <header className='fixed top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60'>
      <div className='flex h-14 items-center justify-between'>
        {/* Logo + Project Switcher - 左侧对齐 */}
        <div className='flex items-center gap-2 pl-6'>
          {/* Sidebar Toggle - 与侧边栏图标垂直对齐 */}
          <SidebarTrigger className='size-8 -ml-4' />
          
          {/* Logo */}
          <div className='flex items-center gap-2'>
            <div className='flex size-8 items-center justify-center rounded overflow-hidden shrink-0'>
              {brandSettings?.brandLogo ? (
                <img
                  src={brandSettings.brandLogo}
                  alt='Brand Logo'
                  width={24}
                  height={24}
                  className='size-8 object-cover'
                  onError={(e) => {
                    e.currentTarget.src = '/logo.jpg'
                  }}
                />
              ) : (
                <img
                  src='/logo.jpg'
                  alt='Default Logo'
                  width={24}
                  height={24}
                  className='size-8 object-cover'
                />
              )}
            </div>
            <span className='font-semibold text-sm leading-none'>{displayName}</span>
          </div>

          {/* Separator */}
          <div className='h-3.5 w-px bg-border mx-0.5' />

          {/* Project Switcher */}
          <ProjectSwitcher />
        </div>

        {/* 右侧控件 */}
        <div className='flex items-center gap-2 pr-6'>
          <LanguageSwitch />
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </div>
    </header>
  )
}
