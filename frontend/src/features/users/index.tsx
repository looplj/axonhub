import React, { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks/use-debounce'
import { usePermissions } from '@/hooks/usePermissions'
import { LanguageSwitch } from '@/components/language-switch'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { createColumns } from './components/users-columns'
import { UsersDialogs } from './components/users-dialogs'
import { UsersPrimaryButtons } from './components/users-primary-buttons'
import { UsersTable } from './components/users-table'
import UsersProvider from './context/users-context'
import { useUsers } from './data/users'

function UsersContent() {
  const { t } = useTranslation()
  const { userPermissions } = usePermissions()
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)

  // Filter states
  const [nameFilter, setNameFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [roleFilter, setRoleFilter] = useState<string[]>([])

  const debouncedNameFilter = useDebounce(nameFilter, 300)

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(
    () => createColumns(t, userPermissions.canWrite),
    [t, userPermissions.canWrite]
  )

  // Build where clause for API filtering
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {}
    if (debouncedNameFilter) {
      where.firstNameContainsFold = debouncedNameFilter
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter
    }
    if (roleFilter.length > 0) {
      // Note: This would need to be implemented based on the actual user role relationship
      // For now, we'll leave it as a placeholder
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { data, isLoading, error: _error } = useUsers({
    first: pageSize,
    after: cursor,
    where: whereClause,
  })

  // Reset cursor when filters change
  React.useEffect(() => {
    setCursor(undefined)
  }, [debouncedNameFilter, statusFilter, roleFilter])

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursor(data.pageInfo.endCursor)
    }
  }

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage && data?.pageInfo?.startCursor) {
      setCursor(data.pageInfo.startCursor)
    }
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize)
    setCursor(undefined) // Reset to first page
  }

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <UsersTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        loading={isLoading}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        nameFilter={nameFilter}
        statusFilter={statusFilter}
        roleFilter={roleFilter}
        onNameFilterChange={setNameFilter}
        onStatusFilterChange={setStatusFilter}
        onRoleFilterChange={setRoleFilter}
      />
      </div>
  )
}

export default function UsersManagement() {
  const { t } = useTranslation()

  return (
    <UsersProvider>
      <Header fixed>
        {/* <Search /> */}
        <div className='ml-auto flex items-center space-x-4'>
          <LanguageSwitch />
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>
              {t('users.title')}
            </h2>
            <p className='text-muted-foreground'>{t('users.description')}</p>
          </div>
          <UsersPrimaryButtons />
        </div>
        <UsersContent />
      </Main>
      <UsersDialogs />
    </UsersProvider>
  )
}
