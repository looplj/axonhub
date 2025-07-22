import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { columns } from './components/users-columns'
import { UsersDialogs } from './components/users-dialogs'
import { UsersPrimaryButtons } from './components/users-primary-buttons'
import { UsersTable } from './components/users-table'
import UsersProvider from './context/users-context'
import { useUsers } from './data/users'

export default function UsersManagement() {
  const { data: users, isLoading, error } = useUsers()

  return (
    <UsersProvider>
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
            <h2 className='text-2xl font-bold tracking-tight'>用户管理</h2>
            <p className='text-muted-foreground'>
              管理系统用户和权限配置。
            </p>
          </div>
          <UsersPrimaryButtons />
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
            <UsersTable data={users?.edges?.map(edge => edge.node) || []} columns={columns} />
          )}
        </div>
      </Main>
      <UsersDialogs />
    </UsersProvider>
  )
}
