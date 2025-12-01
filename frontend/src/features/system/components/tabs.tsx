'use client'

import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { BrandSettings } from './brand-settings'
import { RetrySettings } from './retry-settings'
import { StorageSettings } from './storage-settings'

type SystemTabKey = 'brand' | 'storage' | 'retry'

interface SystemSettingsTabsProps {
  initialTab?: SystemTabKey
}

export function SystemSettingsTabs({ initialTab }: SystemSettingsTabsProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<SystemTabKey>('brand')

  useEffect(() => {
    if (initialTab) {
      setActiveTab(initialTab)
    }
  }, [initialTab])

  return (
    <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as SystemTabKey)} className='w-full'>
      <TabsList className='grid w-full grid-cols-3'>
        <TabsTrigger value='brand' data-value='brand'>
          {t('system.tabs.brand')}
        </TabsTrigger>
        <TabsTrigger value='retry' data-value='retry'>
          {t('system.tabs.retry')}
        </TabsTrigger>
        <TabsTrigger value='storage' data-value='storage'>
          {t('system.tabs.storage')}
        </TabsTrigger>
      </TabsList>
      <TabsContent value='brand' className='mt-6'>
        <BrandSettings />
      </TabsContent>
      <TabsContent value='storage' className='mt-6'>
        <StorageSettings />
      </TabsContent>
      <TabsContent value='retry' className='mt-6'>
        <RetrySettings />
      </TabsContent>
    </Tabs>
  )
}
