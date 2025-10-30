'use client'

import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { BrandSettings } from './brand-settings'
import { StorageSettings } from './storage-settings'
import { RetrySettings } from './retry-settings'

type SystemTabKey = 'brand' | 'storage' | 'retry'

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
    <Tabs
      value={activeTab}
      onValueChange={(value) => setActiveTab(value as SystemTabKey)}
      className="w-full"
    >
      <TabsList className="grid w-full grid-cols-3">
        <TabsTrigger value="brand">{t('system.tabs.brand')}</TabsTrigger>
        <TabsTrigger value="storage">{t('system.tabs.storage')}</TabsTrigger>
        <TabsTrigger value="retry">{t('system.tabs.retry')}</TabsTrigger>
      </TabsList>
      <TabsContent value="brand" className="mt-6">
        <BrandSettings />
      </TabsContent>
      <TabsContent value="storage" className="mt-6">
        <StorageSettings />
      </TabsContent>
      <TabsContent value="retry" className="mt-6">
        <RetrySettings />
      </TabsContent>
    </Tabs>
  )
}