import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { columns } from './components/channels-columns'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsTable } from './components/channels-table'
import ChannelsProvider from './context/channels-context'
import { useChannels } from './data/channels'

export default function ChannelsManagement() {
  const { data: channels, isLoading, error } = useChannels()

  return (
    <ChannelsProvider>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Channel 管理</h2>
            <p className='text-muted-foreground'>
              管理您的 AI 模型 Channel 和配置。
            </p>
          </div>
          <ChannelsPrimaryButtons />
        </div>
        <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
          {isLoading ? (
            <div className='flex items-center justify-center h-32'>
              <div className='text-muted-foreground'>加载中...</div>
            </div>
          ) : error ? (
            <div className='flex items-center justify-center h-32'>
              <div className='text-destructive'>加载失败: {error.message}</div>
            </div>
          ) : (
            <ChannelsTable data={channels?.edges?.map(edge => edge.node) || []} columns={columns} />
          )}
        </div>
      </Main>
      <ChannelsDialogs />
    </ChannelsProvider>
  )
}