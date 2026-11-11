'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useHorizontalScroll } from '@/hooks/use-horizontal-scroll';
import { SYSTEM_PAGES, type SystemPageKey, type SystemTabKey } from '../pages';

export type { SystemTabKey };

interface SystemSettingsTabsProps {
  page: SystemPageKey;
  initialTab?: string;
}

export function SystemSettingsTabs({ page, initialTab }: SystemSettingsTabsProps) {
  const { t } = useTranslation();
  const { isOwner } = usePermissions();
  const tabs = useMemo(() => SYSTEM_PAGES[page].tabs.filter((tab) => isOwner || !tab.ownerOnly), [page, isOwner]);
  const defaultTab = tabs[0].key;
  const [activeTab, setActiveTab] = useState<SystemTabKey>(defaultTab);
  const tabListRef = useRef<HTMLDivElement>(null);
  const horizontalScrollRef = useHorizontalScroll<HTMLDivElement>();
  const setTabListRef = useCallback(
    (node: HTMLDivElement | null) => {
      tabListRef.current = node;
      horizontalScrollRef(node);
    },
    [horizontalScrollRef]
  );

  const isAvailableTab = useCallback((value: string | undefined): value is SystemTabKey => tabs.some((tab) => tab.key === value), [tabs]);

  useEffect(() => {
    setActiveTab(isAvailableTab(initialTab) ? initialTab : defaultTab);
  }, [initialTab, defaultTab, isAvailableTab]);

  useEffect(() => {
    const tabList = tabListRef.current;
    if (!tabList) return;

    const activeTrigger = Array.from(tabList.querySelectorAll<HTMLElement>('[data-value]')).find(
      (trigger) => trigger.dataset.value === activeTab
    );
    if (!activeTrigger) return;

    const listRect = tabList.getBoundingClientRect();
    const triggerRect = activeTrigger.getBoundingClientRect();
    if (triggerRect.left < listRect.left) {
      tabList.scrollLeft -= listRect.left - triggerRect.left;
    } else if (triggerRect.right > listRect.right) {
      tabList.scrollLeft += triggerRect.right - listRect.right;
    }
  }, [activeTab]);

  return (
    <Tabs
      value={activeTab}
      onValueChange={(value) => {
        if (!isAvailableTab(value)) return;
        setActiveTab(value);
      }}
      className='w-full'
    >
      <TabsList
        ref={setTabListRef}
        className='shadow-soft border-border bg-background flex w-full justify-start overflow-x-auto rounded-2xl border sm:overflow-x-visible [&_[data-slot=tabs-trigger]]:flex-none [&_[data-slot=tabs-trigger]]:shrink-0 sm:[&_[data-slot=tabs-trigger]]:flex-1 sm:[&_[data-slot=tabs-trigger]]:shrink'
      >
        {tabs.map((tab) => (
          <TabsTrigger key={tab.key} value={tab.key} data-value={tab.key}>
            {t(tab.labelKey)}
          </TabsTrigger>
        ))}
      </TabsList>
      <div className='shadow-soft border-border bg-card mt-6 rounded-2xl border p-4 sm:p-6'>
        {tabs.map((tab) => (
          <TabsContent key={tab.key} value={tab.key} className='mt-0 p-0'>
            <tab.component />
          </TabsContent>
        ))}
      </div>
    </Tabs>
  );
}
