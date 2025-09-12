'use client'

import React, { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { BrandSettings } from './brand-settings'
import { StorageSettings } from './storage-settings'

export function SystemSettingsTabs() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('brand')

  return (
    <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
      <TabsList className="grid w-full grid-cols-2">
        <TabsTrigger value="brand">{t('system.tabs.brand')}</TabsTrigger>
        <TabsTrigger value="storage">{t('system.tabs.storage')}</TabsTrigger>
      </TabsList>
      <TabsContent value="brand" className="mt-6">
        <BrandSettings />
      </TabsContent>
      <TabsContent value="storage" className="mt-6">
        <StorageSettings />
      </TabsContent>
    </Tabs>
  )
}