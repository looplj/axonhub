import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks/use-debounce'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { createColumns } from './components/data-storages-columns'
import { DataStorageDialogs } from './components/data-storage-dialogs'
import { DataStoragesPrimaryButtons } from './components/data-storages-primary-buttons'
import { DataStoragesTable } from './components/data-storages-table'
import DataStoragesProvider from './context/data-storages-context'
import { useDataStorages } from './data/data-storages'

function DataStoragesContent() {
  const { t } = useTranslation()
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [nameFilter, setNameFilter] = useState<string>('')
  const [typeFilter, setTypeFilter] = useState<string[]>([])
  const [statusFilter, setStatusFilter] = useState<string[]>([])

  // Debounce the name filter to avoid excessive API calls
  const debouncedNameFilter = useDebounce(nameFilter, 300)

  // Build where clause with filters
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {}
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter
    }
    if (typeFilter.length > 0) {
      where.typeIn = typeFilter
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter
    } else {
      // By default, only show active data storages
      where.statusIn = ['active']
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { data } = useDataStorages({
    first: pageSize,
    after: cursor,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
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
    setCursor(undefined) // Reset to first page when filter changes
  }

  const handleTypeFilterChange = (filters: string[]) => {
    setTypeFilter(filters)
    setCursor(undefined) // Reset to first page when filter changes
  }

  const handleStatusFilterChange = (filters: string[]) => {
    setStatusFilter(filters)
    setCursor(undefined) // Reset to first page when filter changes
  }

  const columns = createColumns(t)

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <DataStoragesTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        typeFilter={typeFilter}
        statusFilter={statusFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onNameFilterChange={handleNameFilterChange}
        onTypeFilterChange={handleTypeFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
      />
    </div>
  )
}

export default function DataStoragesManagement() {
  const { t } = useTranslation()

  return (
    <DataStoragesProvider>
      <Header fixed>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>
              {t('dataStorages.title')}
            </h2>
            <p className='text-muted-foreground'>
              {t('dataStorages.description')}
            </p>
            <p className='text-sm text-muted-foreground'>
              {t('dataStorages.llmStorageHint')}
            </p>
          </div>
          <DataStoragesPrimaryButtons />
        </div>
        <DataStoragesContent />
      </Main>
      <DataStorageDialogs />
    </DataStoragesProvider>
  )
}
