'use client';

import React, { useCallback } from 'react';
import { BookOpen, List, Loader2, RefreshCw, Route } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  SETTINGS_DIALOG_CONTENT_CLASS,
  SettingsDialogBody,
  SettingsSection,
  SettingsSubOptions,
  SettingsSwitchRow,
} from '@/components/settings-dialog-layout';
import {
  useCatalogSettings,
  useModelSettings,
  useUpdateCatalogSettings,
  useUpdateModelSettings,
  type UpdateModelSettingsInput,
} from '@/features/system/data/system';
import { useModels } from '../context/models-context';
import { useRefreshProvidersCatalog } from '../data/providers';

const MIN_CATALOG_REFRESH_SECONDS = 60;
const MAX_CATALOG_REFRESH_SECONDS = 604800;

function isValidRefreshSeconds(value: number): boolean {
  return Number.isInteger(value) && value >= MIN_CATALOG_REFRESH_SECONDS && value <= MAX_CATALOG_REFRESH_SECONDS;
}

function clampRefreshSeconds(value: number): number {
  return Math.min(MAX_CATALOG_REFRESH_SECONDS, Math.max(MIN_CATALOG_REFRESH_SECONDS, value));
}

type SettingsSectionKey = 'routing' | 'modelList' | 'catalog';

const SECTIONS: { key: SettingsSectionKey; titleKey: string; descriptionKey: string; icon: React.ElementType }[] = [
  {
    key: 'routing',
    titleKey: 'models.dialogs.settings.sections.routing',
    descriptionKey: 'models.dialogs.settings.sections.routingDescription',
    icon: Route,
  },
  {
    key: 'modelList',
    titleKey: 'models.dialogs.settings.sections.modelList',
    descriptionKey: 'models.dialogs.settings.sections.modelListDescription',
    icon: List,
  },
  { key: 'catalog', titleKey: 'system.catalog.title', descriptionKey: 'system.catalog.description', icon: BookOpen },
];

export function ModelSettingsDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useModels();
  const { data: settings, isLoading } = useModelSettings();
  const { data: catalogSettings, isLoading: isLoadingCatalog } = useCatalogSettings();
  const updateModelSettings = useUpdateModelSettings();
  const updateCatalogSettings = useUpdateCatalogSettings();
  const refreshCatalog = useRefreshProvidersCatalog();

  const isOpen = open === 'settings';
  const [activeSection, setActiveSection] = React.useState<SettingsSectionKey>('routing');

  const [fallbackEnabled, setFallbackEnabled] = React.useState(false);
  const [queryAllChannelModels, setQueryAllChannelModels] = React.useState(false);
  const [defaultModelAPIIncludeAll, setDefaultModelAPIIncludeAll] = React.useState(false);
  const [autoReasoningEffort, setAutoReasoningEffort] = React.useState(false);
  const [modelBlacklistRegex, setModelBlacklistRegex] = React.useState('');
  const [hideUnroutableModelsInList, setHideUnroutableModelsInList] = React.useState(false);
  const [catalogUpstreamURL, setCatalogUpstreamURL] = React.useState('');
  const [catalogRefreshSeconds, setCatalogRefreshSeconds] = React.useState(3600);

  // Reset the form from the server state whenever the dialog opens.
  React.useEffect(() => {
    if (isOpen) {
      setActiveSection('routing');
    }
  }, [isOpen]);

  React.useEffect(() => {
    if (settings && isOpen) {
      setFallbackEnabled(settings.fallbackToChannelsOnModelNotFound);
      setQueryAllChannelModels(settings.queryAllChannelModels);
      setDefaultModelAPIIncludeAll(settings.defaultModelAPIIncludeAll);
      setAutoReasoningEffort(settings.autoReasoningEffort);
      setModelBlacklistRegex(settings.modelBlacklistRegex ?? '');
      setHideUnroutableModelsInList(settings.hideUnroutableModelsInList);
    }
  }, [settings, isOpen]);

  React.useEffect(() => {
    if (catalogSettings && isOpen) {
      setCatalogUpstreamURL(catalogSettings.upstreamURL);
      setCatalogRefreshSeconds(catalogSettings.refreshSeconds);
    }
  }, [catalogSettings, isOpen]);

  const modelSettingsChanged =
    !!settings &&
    (settings.fallbackToChannelsOnModelNotFound !== fallbackEnabled ||
      settings.queryAllChannelModels !== queryAllChannelModels ||
      settings.defaultModelAPIIncludeAll !== defaultModelAPIIncludeAll ||
      settings.autoReasoningEffort !== autoReasoningEffort ||
      (settings.modelBlacklistRegex ?? '') !== modelBlacklistRegex ||
      settings.hideUnroutableModelsInList !== hideUnroutableModelsInList);

  const catalogChanged =
    !!catalogSettings &&
    (catalogSettings.upstreamURL !== catalogUpstreamURL.trim() || catalogSettings.refreshSeconds !== catalogRefreshSeconds);
  const catalogRefreshValid = isValidRefreshSeconds(catalogRefreshSeconds);

  const isSaving = updateModelSettings.isPending || updateCatalogSettings.isPending;

  // Each settings group is saved only when it changed, so an untouched group
  // does not produce an extra request or toast.
  const handleSave = useCallback(async () => {
    if (modelSettingsChanged) {
      const input: UpdateModelSettingsInput = {
        fallbackToChannelsOnModelNotFound: fallbackEnabled,
        queryAllChannelModels: queryAllChannelModels,
        defaultModelAPIIncludeAll: defaultModelAPIIncludeAll,
        autoReasoningEffort: autoReasoningEffort,
        modelBlacklistRegex: modelBlacklistRegex,
        hideUnroutableModelsInList: hideUnroutableModelsInList,
        developerSettings: settings?.developerSettings || [],
      };
      await updateModelSettings.mutateAsync(input);
    }
    if (catalogChanged) {
      await updateCatalogSettings.mutateAsync({
        upstreamURL: catalogUpstreamURL.trim(),
        refreshSeconds: clampRefreshSeconds(catalogRefreshSeconds),
      });
    }
    setOpen(null);
  }, [
    modelSettingsChanged,
    catalogChanged,
    updateModelSettings,
    updateCatalogSettings,
    fallbackEnabled,
    queryAllChannelModels,
    defaultModelAPIIncludeAll,
    autoReasoningEffort,
    modelBlacklistRegex,
    hideUnroutableModelsInList,
    settings?.developerSettings,
    catalogUpstreamURL,
    catalogRefreshSeconds,
    setOpen,
  ]);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className={SETTINGS_DIALOG_CONTENT_CLASS}>
        <DialogHeader className='shrink-0'>
          <DialogTitle>{t('models.dialogs.settings.title')}</DialogTitle>
          <DialogDescription>{t('models.dialogs.settings.description')}</DialogDescription>
        </DialogHeader>

        {isLoading || isLoadingCatalog ? (
          <div className='flex flex-1 items-center justify-center'>
            <Loader2 className='h-8 w-8 animate-spin' />
          </div>
        ) : (
          <SettingsDialogBody
            items={SECTIONS.map((section) => ({ key: section.key, title: t(section.titleKey), icon: section.icon }))}
            active={activeSection}
            onSelect={setActiveSection}
          >
            {activeSection === 'routing' && (
              <SettingsSection
                title={t('models.dialogs.settings.sections.routing')}
                description={t('models.dialogs.settings.sections.routingDescription')}
              >
                <SettingsSwitchRow
                  id='fallback-enabled'
                  label={t('models.dialogs.settings.fallbackToChannels.label')}
                  description={t('models.dialogs.settings.fallbackToChannels.description')}
                  checked={fallbackEnabled}
                  onCheckedChange={setFallbackEnabled}
                  disabled={isSaving}
                />
                <SettingsSwitchRow
                  id='auto-reasoning-effort'
                  label={t('models.dialogs.settings.autoReasoningEffort.label')}
                  description={t('models.dialogs.settings.autoReasoningEffort.description')}
                  checked={autoReasoningEffort}
                  onCheckedChange={setAutoReasoningEffort}
                  disabled={isSaving}
                />
              </SettingsSection>
            )}

            {activeSection === 'modelList' && (
              <SettingsSection
                title={t('models.dialogs.settings.sections.modelList')}
                description={t('models.dialogs.settings.sections.modelListDescription')}
              >
                <SettingsSwitchRow
                  id='query-all-channel-models'
                  label={t('models.dialogs.settings.queryAllChannelModels.label')}
                  description={t('models.dialogs.settings.queryAllChannelModels.description')}
                  checked={queryAllChannelModels}
                  onCheckedChange={setQueryAllChannelModels}
                  disabled={isSaving}
                >
                  {queryAllChannelModels && (
                    <SettingsSubOptions>
                      <Label htmlFor='model-blacklist-regex' className='text-sm font-medium'>
                        {t('models.dialogs.settings.modelBlacklistRegex.label')}
                      </Label>
                      <Input
                        id='model-blacklist-regex'
                        value={modelBlacklistRegex}
                        onChange={(e) => setModelBlacklistRegex(e.target.value)}
                        placeholder={t('models.dialogs.settings.modelBlacklistRegex.placeholder')}
                        disabled={isSaving}
                        className='font-mono text-sm'
                      />
                      <p className='text-muted-foreground text-xs leading-relaxed'>
                        {t('models.dialogs.settings.modelBlacklistRegex.description')}
                      </p>
                    </SettingsSubOptions>
                  )}
                </SettingsSwitchRow>
                <SettingsSwitchRow
                  id='hide-unroutable-models-in-list'
                  label={t('models.dialogs.settings.hideUnroutableModelsInList.label')}
                  description={t('models.dialogs.settings.hideUnroutableModelsInList.description')}
                  checked={hideUnroutableModelsInList}
                  onCheckedChange={setHideUnroutableModelsInList}
                  disabled={isSaving}
                />
                <SettingsSwitchRow
                  id='default-model-api-include-all'
                  label={t('models.dialogs.settings.defaultModelAPIIncludeAll.label')}
                  description={t('models.dialogs.settings.defaultModelAPIIncludeAll.description')}
                  checked={defaultModelAPIIncludeAll}
                  onCheckedChange={setDefaultModelAPIIncludeAll}
                  disabled={isSaving}
                />
              </SettingsSection>
            )}

            {activeSection === 'catalog' && (
              <SettingsSection title={t('system.catalog.title')} description={t('system.catalog.description')}>
                <div className='space-y-4 p-4'>
                  <div className='space-y-1.5'>
                    <Label htmlFor='catalog-upstream-url' className='text-sm font-medium'>
                      {t('system.catalog.upstreamURL.label')}
                    </Label>
                    <Input
                      id='catalog-upstream-url'
                      value={catalogUpstreamURL}
                      onChange={(event) => setCatalogUpstreamURL(event.target.value)}
                      placeholder='https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json'
                      disabled={isSaving}
                      className='font-mono text-sm'
                    />
                    <p className='text-muted-foreground text-xs'>{t('system.catalog.upstreamURL.description')}</p>
                  </div>
                  <div className='flex flex-wrap items-end justify-between gap-3'>
                    <div className='space-y-1.5'>
                      <Label htmlFor='catalog-refresh-seconds' className='text-sm font-medium'>
                        {t('system.catalog.refreshSeconds.label')}
                      </Label>
                      <Input
                        id='catalog-refresh-seconds'
                        type='number'
                        min={MIN_CATALOG_REFRESH_SECONDS}
                        max={MAX_CATALOG_REFRESH_SECONDS}
                        step={1}
                        value={Number.isInteger(catalogRefreshSeconds) ? catalogRefreshSeconds : ''}
                        onChange={(event) => {
                          const parsed = Number.parseInt(event.target.value, 10);
                          setCatalogRefreshSeconds(Number.isNaN(parsed) ? Number.NaN : parsed);
                        }}
                        onBlur={() => {
                          if (Number.isInteger(catalogRefreshSeconds)) {
                            setCatalogRefreshSeconds(clampRefreshSeconds(catalogRefreshSeconds));
                          }
                        }}
                        disabled={isSaving}
                        className='w-40'
                      />
                      <p className='text-muted-foreground text-xs'>{t('system.catalog.refreshSeconds.description')}</p>
                    </div>
                    <Button variant='outline' size='sm' onClick={() => refreshCatalog.mutate()} disabled={refreshCatalog.isPending}>
                      {refreshCatalog.isPending ? (
                        <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                      ) : (
                        <RefreshCw className='mr-2 h-4 w-4' />
                      )}
                      {t('system.catalog.refreshNow')}
                    </Button>
                  </div>
                </div>
              </SettingsSection>
            )}
          </SettingsDialogBody>
        )}

        <DialogFooter className='flex shrink-0 flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-center sm:gap-2'>
          <Button variant='outline' onClick={handleClose} disabled={isSaving} className='h-10 w-full sm:h-9 sm:w-auto'>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            onClick={handleSave}
            disabled={isSaving || isLoading || isLoadingCatalog || !catalogRefreshValid}
            className='h-10 w-full sm:h-9 sm:w-auto'
          >
            {isSaving ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('common.buttons.saving')}
              </>
            ) : (
              t('common.buttons.save')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
