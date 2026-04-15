'use client';

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { AboutSettings } from './about-settings';
import { BrandSettings } from './brand-settings';
import { DiagnosticsSettings } from './diagnostics-settings';
import { GeneralSettings } from './general-settings';
import { RetrySettings } from './retry-settings';
import { StorageSettings } from './storage-settings';
import { BackupSettings } from './backup-settings';
import { ProxyPresetsSettings } from './proxy-presets-settings';
import { WebhookSettings } from './webhook-settings';
import { usePermissions } from '@/hooks/usePermissions';

type SystemTabKey = 'general' | 'brand' | 'storage' | 'retry' | 'webhook' | 'proxy' | 'backup' | 'diagnostics' | 'about';

interface SystemSettingsTabsProps {
  initialTab?: SystemTabKey;
}

export function SystemSettingsTabs({ initialTab }: SystemSettingsTabsProps) {
  const { t } = useTranslation();
  const { isOwner } = usePermissions();
  const [activeTab, setActiveTab] = useState<SystemTabKey>('general');

  useEffect(() => {
    if (!initialTab) {
      return;
    }

    if (!isOwner && (initialTab === 'backup' || initialTab === 'diagnostics')) {
      setActiveTab('general');
      return;
    }

    setActiveTab(initialTab);
  }, [initialTab, isOwner]);

  return (
    <Tabs
      value={activeTab}
      onValueChange={(value) => {
        const nextTab = value as SystemTabKey;
        if (!isOwner && (nextTab === 'backup' || nextTab === 'diagnostics')) {
          setActiveTab('general');
          return;
        }
        setActiveTab(nextTab);
      }}
      className='w-full'
    >
      <TabsList className='shadow-soft border-border bg-background flex w-full rounded-2xl border overflow-x-auto scrollbar-hide'>
        <TabsTrigger value='general' data-value='general'>
          {t('system.tabs.general')}
        </TabsTrigger>
        <TabsTrigger value='brand' data-value='brand'>
          {t('system.tabs.brand')}
        </TabsTrigger>
        <TabsTrigger value='retry' data-value='retry'>
          {t('system.tabs.retry')}
        </TabsTrigger>
        <TabsTrigger value='webhook' data-value='webhook'>
          {t('system.tabs.webhook')}
        </TabsTrigger>
        <TabsTrigger value='storage' data-value='storage'>
          {t('system.tabs.storage')}
        </TabsTrigger>
        <TabsTrigger value='proxy' data-value='proxy'>
          {t('system.tabs.proxy')}
        </TabsTrigger>
        {isOwner && (
          <TabsTrigger value='diagnostics' data-value='diagnostics'>
            {t('system.tabs.diagnostics')}
          </TabsTrigger>
        )}
        {isOwner && (
          <TabsTrigger value='backup' data-value='backup'>
            {t('system.tabs.backup')}
          </TabsTrigger>
        )}
        <TabsTrigger value='about' data-value='about'>
          {t('system.tabs.about')}
        </TabsTrigger>
      </TabsList>
      <div className='shadow-soft border-border bg-card mt-6 rounded-2xl border p-4 sm:p-6'>
        <TabsContent value='general' className='mt-0 p-0'>
          <GeneralSettings />
        </TabsContent>
        <TabsContent value='brand' className='mt-0 p-0'>
          <BrandSettings />
        </TabsContent>
        <TabsContent value='storage' className='mt-0 p-0'>
          <StorageSettings />
        </TabsContent>
        <TabsContent value='retry' className='mt-0 p-0'>
          <RetrySettings />
        </TabsContent>
        <TabsContent value='webhook' className='mt-0 p-0'>
          <WebhookSettings />
        </TabsContent>
        <TabsContent value='proxy' className='mt-0 p-0'>
          <ProxyPresetsSettings />
        </TabsContent>
        {isOwner && (
          <TabsContent value='diagnostics' className='mt-0 p-0'>
            <DiagnosticsSettings />
          </TabsContent>
        )}
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
