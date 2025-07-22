import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { apiKeysColumns } from './components/apikeys-columns'
import { ApiKeysDialogs } from './components/apikeys-dialogs'
import { ApiKeysPrimaryButtons } from './components/apikeys-primary-buttons'
import { ApiKeysTable } from './components/apikeys-table'
import { ApiKeysProvider } from './context/apikeys-context'
import { useApiKeys } from './data/apikeys'

export function ApiKeysManagement() {
  const { data: apiKeys, isLoading, error } = useApiKeys()

  if (error) {
    return (
      <div className='flex h-full items-center justify-center'>
        <div className='text-center'>
          <h2 className='text-lg font-semibold'>加载失败</h2>
          <p className='text-muted-foreground'>无法加载 API Keys 数据</p>
        </div>
      </div>
    )
  }

  return (
    <ApiKeysProvider>
      <Header title='API Keys'>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>
      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>API Key 管理</h2>
            <p className='text-muted-foreground'>管理您的 API Key 。</p>
          </div>
          <ApiKeysPrimaryButtons />
        </div>
        <ApiKeysTable
          columns={apiKeysColumns}
          data={apiKeys?.edges?.map((edge) => edge.node) || []}
          isLoading={isLoading}
        />
      </Main>
      <ApiKeysDialogs />
    </ApiKeysProvider>
  )
}

export default ApiKeysManagement
