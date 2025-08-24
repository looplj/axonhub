import React, { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks/use-debounce'
import { LanguageSwitch } from '@/components/language-switch'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { createColumns } from './components/apikeys-columns'
import { ApiKeysDialogs } from './components/apikeys-dialogs'
import { ApiKeysPrimaryButtons } from './components/apikeys-primary-buttons'
import { ApiKeysTable } from './components/apikeys-table'
import ApiKeysProvider from './context/apikeys-context'
import { useApiKeys } from './data/apikeys'

function ApiKeysContent() {
  const { t } = useTranslation()
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)

  // Filter states - following the same pattern as roles and users
  const [nameFilter, setNameFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [userFilter, setUserFilter] = useState<string[]>([])

  const debouncedNameFilter = useDebounce(nameFilter, 300)

  // Build where clause for API filtering
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {}
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter
    }
    if (userFilter.length > 0 && userFilter[0]) {
      where.userID = userFilter[0] // API expects single userID
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { data, isLoading, error } = useApiKeys({
    first: pageSize,
    after: cursor,
    where: whereClause,
  })

  // Reset cursor when filters change
  React.useEffect(() => {
    setCursor(undefined)
  }, [debouncedNameFilter, statusFilter, userFilter])

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

  const handleResetFilters = () => {
    setNameFilter('')
    setStatusFilter([])
    setUserFilter([])
    setCursor(undefined) // Reset to first page
  }

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <ApiKeysTable
        data={data?.edges?.map((edge) => edge.node) || []}
        loading={isLoading}

        columns={createColumns(t)}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        statusFilter={statusFilter}
        userFilter={userFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onNameFilterChange={setNameFilter}
        onStatusFilterChange={setStatusFilter}
        onUserFilterChange={setUserFilter}
        onResetFilters={handleResetFilters}
      />
    </div>
  )
}

export default function ApiKeysManagement() {
  const { t } = useTranslation()

  return (
    <ApiKeysProvider>
      <Header fixed>
        <Search />
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
              {t('apikeys.title')}
            </h2>
            <p className='text-muted-foreground'>{t('apikeys.description')}</p>
          </div>
          <ApiKeysPrimaryButtons />
        </div>
        <ApiKeysContent />
      </Main>
      <ApiKeysDialogs />
    </ApiKeysProvider>
  )
}
