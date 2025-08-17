import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { LanguageSwitch } from '@/components/language-switch'
import { createColumns } from './components/channels-columns'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsTable } from './components/channels-table'
import ChannelsProvider from './context/channels-context'
import { useChannels } from './data/channels'

function ChannelsContent() {
  const { t } = useTranslation()
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  
  const { data, isLoading, error } = useChannels({
    first: pageSize,
    after: cursor,
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

  const columns = createColumns(t)

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      {isLoading ? (
        <div className='flex items-center justify-center h-32'>
          <div className='text-muted-foreground'>{t('channels.loading')}</div>
        </div>
      ) : error ? (
        <div className='flex items-center justify-center h-32'>
          <div className='text-destructive'>{t('channels.loadError')} {error.message}</div>
        </div>
      ) : (
        <ChannelsTable 
          data={data?.edges?.map(edge => edge.node) || []} 
          columns={columns}
          pageInfo={data?.pageInfo}
          pageSize={pageSize}
          totalCount={data?.totalCount}
          onNextPage={handleNextPage}
          onPreviousPage={handlePreviousPage}
          onPageSizeChange={handlePageSizeChange}
        />
      )}
    </div>
  )
}

export default function ChannelsManagement() {
  const { t } = useTranslation()
  
  return (
    <ChannelsProvider>
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
            <h2 className='text-2xl font-bold tracking-tight'>{t('channels.title')}</h2>
            <p className='text-muted-foreground'>
              {t('channels.description')}
            </p>
          </div>
          <ChannelsPrimaryButtons />
        </div>
        <ChannelsContent />
      </Main>
      <ChannelsDialogs />
    </ChannelsProvider>
  )
}