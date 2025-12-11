'use client'

import { useTranslation } from 'react-i18next'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { SystemSettingsTabs } from './components/tabs'
import SystemProvider from './context/system-context'

type SystemTabKey = 'brand' | 'storage' | 'retry' | 'about'

interface SystemContentProps {
  initialTab?: SystemTabKey
}

function SystemContent({ initialTab }: SystemContentProps) {
  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <SystemSettingsTabs initialTab={initialTab} />
    </div>
  )
}

interface SystemManagementProps {
  initialTab?: SystemTabKey
}

export default function SystemManagement({ initialTab }: SystemManagementProps) {
  const { t } = useTranslation()
  
  return (
    <SystemProvider>
      <Header fixed></Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div id="system-title">
            <h2 className='text-2xl font-bold tracking-tight'>{t('system.title')}</h2>
            <p className='text-muted-foreground'>
              {t('system.description')}
            </p>
          </div>
        </div>
        <SystemContent initialTab={initialTab} />
      </Main>
    </SystemProvider>
  )
}