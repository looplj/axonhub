import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
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
  const [nameFilter, setNameFilter] = useState<string>('')
  const [userFilter, setUserFilter] = useState<string>('')

  // Debounce the name filter to avoid excessive API calls
  const debouncedNameFilter = useDebounce(nameFilter, 300)

  // Build where clause with filters
  const whereClause = (() => {
    const where: any = {}
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter
    }
    if (userFilter) {
      where.userID = userFilter
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { data, isLoading, error } = useApiKeys({
    first: pageSize,
    after: cursor,
    where: whereClause,
  })

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

  const handleNameFilterChange = (filter: string) => {
    setNameFilter(filter)
    setCursor(undefined) // Reset to first page when filtering
  }

  const handleUserFilterChange = (filter: string) => {
    setUserFilter(filter)
    setCursor(undefined) // Reset to first page when filtering
  }

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      {isLoading ? (
        <div className='flex h-32 items-center justify-center'>
          <div className='text-muted-foreground'>{t('apikeys.loading')}</div>
        </div>
      ) : error ? (
        <div className='flex h-32 items-center justify-center'>
          <div className='text-destructive'>
            {t('apikeys.loadError')} {error.message}
          </div>
        </div>
      ) : (
        <ApiKeysTable
          data={data?.edges?.map((edge) => edge.node) || []}
          columns={createColumns(t)}
          pageInfo={data?.pageInfo}
          pageSize={pageSize}
          totalCount={data?.totalCount}
          nameFilter={nameFilter}
          userFilter={userFilter}
          onNextPage={handleNextPage}
          onPreviousPage={handlePreviousPage}
          onPageSizeChange={handlePageSizeChange}
          onNameFilterChange={handleNameFilterChange}
          onUserFilterChange={handleUserFilterChange}
        />
      )}
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
