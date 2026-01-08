'use client';

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { AboutSettings } from './about-settings';
import { BrandSettings } from './brand-settings';
import { RetrySettings } from './retry-settings';
import { StorageSettings } from './storage-settings';
import { BackupSettings } from './backup-settings';
import { usePermissions } from '@/hooks/usePermissions';

type SystemTabKey = 'brand' | 'storage' | 'retry' | 'backup' | 'about';

interface SystemSettingsTabsProps {
  initialTab?: SystemTabKey;
}

export function SystemSettingsTabs({ initialTab }: SystemSettingsTabsProps) {
  const { t } = useTranslation();
  const { isOwner } = usePermissions();
  const [activeTab, setActiveTab] = useState<SystemTabKey>('brand');

  useEffect(() => {
    if (initialTab) {
      setActiveTab(initialTab);
    }
  }, [initialTab]);

  return (
    <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as SystemTabKey)} className='w-full'>
      <TabsList className={`shadow-soft border-border bg-background grid w-full rounded-2xl border ${isOwner ? 'grid-cols-5' : 'grid-cols-4'}`}>
        <TabsTrigger value='brand' data-value='brand'>
          {t('system.tabs.brand')}
        </TabsTrigger>
        <TabsTrigger value='retry' data-value='retry'>
          {t('system.tabs.retry')}
        </TabsTrigger>
        <TabsTrigger value='storage' data-value='storage'>
          {t('system.tabs.storage')}
        </TabsTrigger>
        {isOwner && (
          <TabsTrigger value='backup' data-value='backup'>
            {t('system.tabs.backup')}
          </TabsTrigger>
        )}
        <TabsTrigger value='about' data-value='about'>
          {t('system.tabs.about')}
        </TabsTrigger>
      </TabsList>
      <div className='shadow-soft border-border bg-card mt-6 rounded-2xl border p-6'>
        <TabsContent value='brand' className='mt-0 p-0'>
          <BrandSettings />
        </TabsContent>
        <TabsContent value='storage' className='mt-0 p-0'>
          <StorageSettings />
        </TabsContent>
        <TabsContent value='retry' className='mt-0 p-0'>
          <RetrySettings />
        </TabsContent>
        {isOwner && (
          <TabsContent value='backup' className='mt-0 p-0'>
            <BackupSettings />
          </TabsContent>
        )}
        <TabsContent value='about' className='mt-0 p-0'>
          <AboutSettings />
        </TabsContent>
      </div>
    </Tabs>
  );
}
