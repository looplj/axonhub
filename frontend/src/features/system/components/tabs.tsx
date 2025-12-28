'use client'

import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { BrandSettings } from './brand-settings'
import { RetrySettings } from './retry-settings'
import { StorageSettings } from './storage-settings'
import { AboutSettings } from './about-settings'

type SystemTabKey = 'brand' | 'storage' | 'retry' | 'about'

interface SystemSettingsTabsProps {
  initialTab?: SystemTabKey
}

export function SystemSettingsTabs({ initialTab }: SystemSettingsTabsProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<SystemTabKey>('brand')

  useEffect(() => {
    if (initialTab) {
      setActiveTab(initialTab)
    }
  }, [initialTab])

  return (
    <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as SystemTabKey)} className='w-full'>
      <TabsList className='grid w-full grid-cols-4 rounded-2xl shadow-soft border border-white bg-white'>
        <TabsTrigger value='brand' data-value='brand'>
          {t('system.tabs.brand')}
        </TabsTrigger>
        <TabsTrigger value='retry' data-value='retry'>
          {t('system.tabs.retry')}
        </TabsTrigger>
        <TabsTrigger value='storage' data-value='storage'>
          {t('system.tabs.storage')}
        </TabsTrigger>
        <TabsTrigger value='about' data-value='about'>
          {t('system.tabs.about')}
        </TabsTrigger>
      </TabsList>
      <div className='mt-6 rounded-2xl shadow-soft border border-white bg-white p-6'>
        <TabsContent value='brand' className='mt-0 p-0'>
          <BrandSettings />
        </TabsContent>
        <TabsContent value='storage' className='mt-0 p-0'>
          <StorageSettings />
        </TabsContent>
        <TabsContent value='retry' className='mt-0 p-0'>
          <RetrySettings />
        </TabsContent>
        <TabsContent value='about' className='mt-0 p-0'>
          <AboutSettings />
        </TabsContent>
      </div>
    </Tabs>
  )
}
